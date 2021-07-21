package models

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

// This subsection of models provides structures related to the
// environment variables of applications.

// Show Response
type EnvVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Set Request, List Response
type EnvVariableList []EnvVariable

// Unset Request, Match Response
type EnvVarnameList []string

// Implement the Sort interface for EV definition slices

func (evl EnvVariableList) Len() int {
	return len(evl)
}

func (evl EnvVariableList) Swap(i, j int) {
	evl[i], evl[j] = evl[j], evl[i]
}

func (evl EnvVariableList) Less(i, j int) bool {
	return evl[i].Name < evl[j].Name
}

func (evl EnvVariableList) ToEnvVarArray(appRef AppRef) []v1.EnvVar {
	deploymentEnvironment := []v1.EnvVar{
		{
			Name:  "PORT",
			Value: "8080",
		},
	}

	for _, ev := range evl {
		deploymentEnvironment = append(deploymentEnvironment, v1.EnvVar{
			Name: ev.Name,
			ValueFrom: &v1.EnvVarSource{
				SecretKeyRef: &v1.SecretKeySelector{
					Key: ev.Name,
					LocalObjectReference: v1.LocalObjectReference{
						Name: appRef.EnvSecret(),
					},
				},
			},
		})
	}

	return deploymentEnvironment
}

func (evl EnvVariableList) StagingEnvArray() []string {
	stagingVariables := []string{}

	for _, ev := range evl {
		stagingVariables = append(stagingVariables, fmt.Sprintf("%s=%s", ev.Name, ev.Value))
	}

	return stagingVariables
}
