package models

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
)

// This subsection of models provides structures related to the
// environment variables of applications.

// EnvVariable represents the Show Response for a single environment variable
type EnvVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// EnvVariableList is a collection of EVs, it is used for Set Requests, and as List Responses
type EnvVariableList []EnvVariable

// EnvVarnameList is a collection of EV names, it is used for Unset Requests, and as Match Responses
type EnvVarnameList []string

// Implement the Sort interface for EV definition slices

// Len (Sort interface) returns the length of the EnvVariableList
func (evl EnvVariableList) Len() int {
	return len(evl)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the EnvVariableList
func (evl EnvVariableList) Swap(i, j int) {
	evl[i], evl[j] = evl[j], evl[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the EnvVariableList and returns true if the condition
// holds, and else false.
func (evl EnvVariableList) Less(i, j int) bool {
	return evl[i].Name < evl[j].Name
}

// ToEnvVarArray converts the collection of environment variables for
// the referenced application, as a combination of standard variables
// and the user-specified variables. The result is used to make the
// application's environment available to the initial deployment
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

// StagingEnvArray returns the collection of environment variables and
// their values in a form suitable for injection into the Tekton
// staging of an application.
func (evl EnvVariableList) StagingEnvArray() []string {
	stagingVariables := []string{}

	for _, ev := range evl {
		stagingVariables = append(stagingVariables, fmt.Sprintf("%s=%s", ev.Name, ev.Value))
	}

	return stagingVariables
}
