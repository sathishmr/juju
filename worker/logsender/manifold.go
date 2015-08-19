// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package logsender

import (
	"github.com/juju/juju/agent"
	"github.com/juju/juju/feature"
	"github.com/juju/juju/worker"
	"github.com/juju/juju/worker/dependency"
	"github.com/juju/juju/worker/util"
)

// ManifoldConfig defines the names of the manifolds on which a
// Manifold will depend.
type ManifoldConfig struct {
	AgentName       string
	APIPasswordName string
	LogSource       LogRecordCh
}

// Manifold returns a dependency manifold that runs a logger
// worker, using the resource names defined in the supplied config.
func Manifold(config ManifoldConfig) dependency.Manifold {
	return dependency.Manifold{
		Inputs: []string{
			config.AgentName,
			config.APIPasswordName,
		},
		Start: func(getResource dependency.GetResourceFunc) (worker.Worker, error) {
			if !feature.IsDbLogEnabled() {
				logger.Warningf("log sender manifold disabled by feature flag")
				return nil, dependency.ErrMissing
			}

			var waiter util.UnblockWaiter
			if err := getResource(config.APIPasswordName, &waiter); err != nil {
				return nil, err
			}
			var agent agent.Agent
			if err := getResource(config.AgentName, &agent); err != nil {
				return nil, err
			}
			return New(config.LogSource, waiter, agent), nil
		},
	}
}
