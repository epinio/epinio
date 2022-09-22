package models

// __ATTENTION__ Functionally identical to EnvVariableMap, EnvVariableList
// Identical structures

import (
	"fmt"
	"sort"
)

// This subsection of models provides structures related to the chart values of applications.

// AppSetting represents the Show Response for a chart value variable
type AppSetting struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// AppSettingList is a collection of chart value assignments
type AppSettingList []AppSetting

// AppSettings is a collection of key/value pairs describing the user's chosen settings with which
// to configure the helm chart referenced by the application's appchart.
type AppSettings map[string]string

func (cvm AppSettings) List() AppSettingList {
	result := AppSettingList{}
	for name, value := range cvm {
		result = append(result, AppSetting{
			Name:  name,
			Value: value,
		})
	}
	sort.Sort(result)
	return result
}

// Implement the Sort interface for CV definition slices

// Len (Sort interface) returns the length of the AppSettingList
func (cvl AppSettingList) Len() int {
	return len(cvl)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the AppSettingList
func (cvl AppSettingList) Swap(i, j int) {
	cvl[i], cvl[j] = cvl[j], cvl[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the AppSettingList and returns true if the condition
// holds, and else false.
func (cvl AppSettingList) Less(i, j int) bool {
	return cvl[i].Name < cvl[j].Name
}

func (cvl AppSettingList) Assignments() []string {
	assignments := []string{}

	for _, cv := range cvl {
		assignments = append(assignments, fmt.Sprintf(`{"name":"%s","value":"%s"}`,
			cv.Name, cv.Value))
	}

	return assignments
}
