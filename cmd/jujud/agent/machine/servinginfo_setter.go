// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package machine

import (
	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"
	worker "gopkg.in/juju/worker.v1"

	coreagent "github.com/juju/juju/agent"
	apiagent "github.com/juju/juju/api/agent"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/worker/dependency"
)

// ServingInfoSetterConfig provides the dependencies for the
// servingInfoSetter manifold.
type ServingInfoSetterConfig struct {
	AgentName     string
	APICallerName string
}

// ServingInfoSetterManifold defines a simple start function which
// runs after the API connection has come up. If the machine agent is
// a controller, it grabs the state serving info over the API and
// records it to agent configuration, and then stops.
func ServingInfoSetterManifold(config ServingInfoSetterConfig) dependency.Manifold {
	return dependency.Manifold{
		Inputs: []string{
			config.AgentName,
			config.APICallerName,
		},
		Start: func(context dependency.Context) (worker.Worker, error) {
			// Get the agent.
			var agent coreagent.Agent
			if err := context.Get(config.AgentName, &agent); err != nil {
				return nil, err
			}

			// Grab the tag and ensure that it's for a machine.
			tag, ok := agent.CurrentConfig().Tag().(names.MachineTag)
			if !ok {
				return nil, errors.New("agent's tag is not a machine tag")
			}

			// Get API connection.
			var apiCaller base.APICaller
			if err := context.Get(config.APICallerName, &apiCaller); err != nil {
				return nil, err
			}
			apiState, err := apiagent.NewState(apiCaller)
			if err != nil {
				return nil, errors.Trace(err)
			}

			// If the machine needs State, grab the state serving info
			// over the API and write it to the agent configuration.
			if controller, err := isController(apiState, tag); err != nil {
				return nil, errors.Annotate(err, "checking controller status")
			} else if !controller {
				// Not a controller, nothing to do.
				return nil, dependency.ErrUninstall
			}

			info, err := apiState.StateServingInfo()
			if err != nil {
				return nil, errors.Annotate(err, "getting state serving info")
			}
			err = agent.ChangeConfig(func(config coreagent.ConfigSetter) error {
				existing, hasInfo := config.StateServingInfo()
				if hasInfo {
					// Use the existing cert and key as they appear to
					// have been already updated by the cert updater
					// worker to have this machine's IP address as
					// part of the cert. This changed cert is never
					// put back into the database, so it isn't
					// reflected in the copy we have got from
					// apiState.
					info.Cert = existing.Cert
					info.PrivateKey = existing.PrivateKey
				}
				config.SetStateServingInfo(info)
				return nil
			})
			if err != nil {
				return nil, errors.Trace(err)
			}

			// All is well - we're done (no actual worker is actually returned).
			return nil, dependency.ErrUninstall
		},
	}
}

func isController(apiState *apiagent.State, tag names.MachineTag) (bool, error) {
	machine, err := apiState.Entity(tag)
	if err != nil {
		return false, errors.Trace(err)
	}
	for _, job := range machine.Jobs() {
		if job.NeedsState() {
			return true, nil
		}
	}
	return false, nil
}
