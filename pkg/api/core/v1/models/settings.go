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

// __ATTENTION__ Functionally identical to EnvVariableMap, EnvVariableList
// Identical structures

import (
	"fmt"
	"sort"
)

// This subsection of models provides structures related to the chart values of applications.

// ChartValueSetting represents the Show Response for a chart value variable
type ChartValueSetting struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ChartValueSettingList is a collection of chart value assignments
type ChartValueSettingList []ChartValueSetting

// ChartValueSettings is a collection of key/value pairs describing the user's chosen settings with which
// to configure the helm chart referenced by the application's appchart.
type ChartValueSettings map[string]string

func (cvm ChartValueSettings) List() ChartValueSettingList {
	result := ChartValueSettingList{}
	for name, value := range cvm {
		result = append(result, ChartValueSetting{
			Name:  name,
			Value: value,
		})
	}
	sort.Sort(result)
	return result
}

// Implement the Sort interface for CV definition slices

// Len (Sort interface) returns the length of the ChartValueSettingList
func (cvl ChartValueSettingList) Len() int {
	return len(cvl)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the ChartValueSettingList
func (cvl ChartValueSettingList) Swap(i, j int) {
	cvl[i], cvl[j] = cvl[j], cvl[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the ChartValueSettingList and returns true if the condition
// holds, and else false.
func (cvl ChartValueSettingList) Less(i, j int) bool {
	return cvl[i].Name < cvl[j].Name
}

func (cvl ChartValueSettingList) Assignments() []string {
	assignments := []string{}

	for _, cv := range cvl {
		assignments = append(assignments, fmt.Sprintf(`{"name":"%s","value":"%s"}`,
			cv.Name, cv.Value))
	}

	return assignments
}
