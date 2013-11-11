// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package container

import (
	"launchpad.net/juju-core/environs/cloudinit"
	"launchpad.net/juju-core/instance"
)

// ManagerConfig contains the initialization parameters for the ContainerManager.
// The name of the manager is used to namespace the containers on the machine.
type ManagerConfig struct {
	Name   string
	LogDir string
}

// Manager is responsible for starting containers, and stopping and listing
// containers that it has started.
type Manager interface {
	// StartContainer creates and starts a new lxc container for the specified machine.
	StartContainer(
		machineConfig *cloudinit.MachineConfig,
		series string,
		network *NetworkConfig) (instance.Instance, error)
	// StopContainer stops and destroyes the lxc container identified by Instance.
	StopContainer(instance.Instance) error
	// ListContainers return a list of containers that have been started by
	// this manager.
	ListContainers() ([]instance.Instance, error)
}
