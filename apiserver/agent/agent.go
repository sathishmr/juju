// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// The machine package implements the API interfaces
// used by the machine agent.
package agent

import (
	"github.com/juju/errors"
	"github.com/juju/names"

	"github.com/juju/juju/apiserver/common"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/mongo"
	"github.com/juju/juju/state"
)

func init() {
	common.RegisterStandardFacade("Agent", 0, NewAgentAPIV0)
	common.RegisterStandardFacade("Agent", 1, NewAgentAPIV1)
}

// AgentAPIV0 implements the version 0 of the API provided to an agent.
type AgentAPIV0 struct {
	*common.PasswordChanger

	st   *state.State
	auth common.Authorizer
}

// NewAgentAPIV0 returns an object implementing version 0 of the Agent API
// with the given authorizer representing the currently logged in client.
func NewAgentAPIV0(st *state.State, resources *common.Resources, auth common.Authorizer) (*AgentAPIV0, error) {
	// Agents are defined to be any user that's not a client user.
	if !auth.AuthMachineAgent() && !auth.AuthUnitAgent() {
		return nil, common.ErrPerm
	}
	getCanChange := func() (common.AuthFunc, error) {
		return auth.AuthOwner, nil
	}
	return &AgentAPIV0{
		PasswordChanger: common.NewPasswordChanger(st, getCanChange),
		st:              st,
		auth:            auth,
	}, nil
}

func (api *AgentAPIV0) GetEntities(args params.Entities) params.AgentGetEntitiesResults {
	results := params.AgentGetEntitiesResults{
		Entities: make([]params.AgentGetEntitiesResult, len(args.Entities)),
	}
	for i, entity := range args.Entities {
		tag, err := names.ParseTag(entity.Tag)
		if err != nil {
			results.Entities[i].Error = common.ServerError(err)
			continue
		}
		result, err := api.getEntity(tag)
		result.Error = common.ServerError(err)
		results.Entities[i] = result
	}
	return results
}

func (api *AgentAPIV0) getEntity(tag names.Tag) (result params.AgentGetEntitiesResult, err error) {
	// Allow only for the owner agent.
	// Note: having a bulk API call for this is utter madness, given that
	// this check means we can only ever return a single object.
	if !api.auth.AuthOwner(tag) {
		err = common.ErrPerm
		return
	}
	entity0, err := api.st.FindEntity(tag)
	if err != nil {
		return
	}
	entity, ok := entity0.(state.Lifer)
	if !ok {
		err = common.NotSupportedError(tag, "life cycles")
		return
	}
	result.Life = params.Life(entity.Life().String())
	if machine, ok := entity.(*state.Machine); ok {
		result.Jobs = stateJobsToAPIParamsJobs(machine.Jobs())
		result.ContainerType = machine.ContainerType()
	}
	return
}

func (api *AgentAPIV0) StateServingInfo() (result state.StateServingInfo, err error) {
	if !api.auth.AuthEnvironManager() {
		err = common.ErrPerm
		return
	}
	return api.st.StateServingInfo()
}

// MongoIsMaster is called by the IsMaster API call
// instead of mongo.IsMaster. It exists so it can
// be overridden by tests.
var MongoIsMaster = mongo.IsMaster

func (api *AgentAPIV0) IsMaster() (params.IsMasterResult, error) {
	if !api.auth.AuthEnvironManager() {
		return params.IsMasterResult{}, common.ErrPerm
	}

	switch tag := api.auth.GetAuthTag().(type) {
	case names.MachineTag:
		machine, err := api.st.Machine(tag.Id())
		if err != nil {
			return params.IsMasterResult{}, common.ErrPerm
		}

		session := api.st.MongoSession()
		isMaster, err := MongoIsMaster(session, machine)
		return params.IsMasterResult{Master: isMaster}, err
	default:
		return params.IsMasterResult{}, errors.Errorf("authenticated entity is not a Machine")
	}
}

func stateJobsToAPIParamsJobs(jobs []state.MachineJob) []params.MachineJob {
	pjobs := make([]params.MachineJob, len(jobs))
	for i, job := range jobs {
		pjobs[i] = params.MachineJob(job.String())
	}
	return pjobs
}

// AgentAPIV1 implements the version 1 of the API provided to an agent.
type AgentAPIV1 struct {
	*AgentAPIV0
}

// NewAgentAPIV1 returns an object implementing version 1 of the Agent API
// with the given authorizer representing the currently logged in client.
// The functionality is like V0, it only knows the additional machine job
// JobManageNetworking.
func NewAgentAPIV1(st *state.State, resources *common.Resources, auth common.Authorizer) (*AgentAPIV1, error) {
	apiV0, err := NewAgentAPIV0(st, resources, auth)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &AgentAPIV1{apiV0}, nil
}
