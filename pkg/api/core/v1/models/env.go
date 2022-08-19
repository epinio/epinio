package models

// __ATTENTION__ Functionally identical to AppSettings, AppSettingList
// Identical structures

import (
	"fmt"
	"sort"
)

// This subsection of models provides structures related to the
// environment variables of applications.

// EnvVariable represents the Show Response for a single environment variable
type EnvVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// EnvVariableList is a collection of EVs.
type EnvVariableList []EnvVariable

// EnvVariableMap is a collection of EVs as a map. It is used for Set Requests, and as
// List Responses
type EnvVariableMap map[string]string

// EnvVarnameList is a collection of EV names, it is used for Unset Requests, and as Match
// Responses
type EnvVarnameList []string

func (evm EnvVariableMap) List() EnvVariableList {
	result := EnvVariableList{}
	for name, value := range evm {
		result = append(result, EnvVariable{
			Name:  name,
			Value: value,
		})
	}
	sort.Sort(result)
	return result
}

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

func (evl EnvVariableList) Assignments() []string {
	assignments := []string{}

	for _, ev := range evl {
		assignments = append(assignments, fmt.Sprintf(`{"name":"%s","value":"%s"}`,
			ev.Name, ev.Value))
	}

	return assignments
}
