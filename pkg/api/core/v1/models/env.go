// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package models

// __ATTENTION__ Functionally identical to CVSettings, CVSettingList
// Identical structures

import (
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
