package tailer

import (
	"errors"

	v1 "k8s.io/api/core/v1"
)

type ContainerState string

const (
	RUNNING    = "running"
	WAITING    = "waiting"
	TERMINATED = "terminated"
)

func NewContainerState(stateConfig string) (ContainerState, error) {
	if stateConfig == RUNNING {
		return RUNNING, nil
	} else if stateConfig == WAITING {
		return WAITING, nil
	} else if stateConfig == TERMINATED {
		return TERMINATED, nil
	}

	return "", errors.New("containerState should be one of 'running', 'waiting', or 'terminated'")
}

func (stateConfig ContainerState) Match(containerState v1.ContainerState) bool {
	return (stateConfig == RUNNING && containerState.Running != nil) ||
		(stateConfig == WAITING && containerState.Waiting != nil) ||
		(stateConfig == TERMINATED && containerState.Terminated != nil)
}
