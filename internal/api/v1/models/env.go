package models

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

func (ev EnvVariableList) Len() int {
	return len(ev)
}

func (ev EnvVariableList) Swap(i, j int) {
	ev[i], ev[j] = ev[j], ev[i]
}

func (ev EnvVariableList) Less(i, j int) bool {
	return ev[i].Name < ev[j].Name
}
