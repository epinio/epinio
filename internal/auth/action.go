package auth

import (
	"fmt"
	"sort"

	"github.com/epinio/epinio/helpers/routes"
	"github.com/pkg/errors"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v2"

	// embedding
	_ "embed"
)

//go:embed actions.yaml
var actionsYAML []byte

// ActionsMap holds the available actions that can be assigned to a Role
// Call LoadActions to load the actions from the actions.yaml file.
var ActionsMap = make(Actions)

type Actions map[string]Action

// Action defines a possible action that can be performed and the allowed Endpoints.
type Action struct {
	ID        string     `yaml:"id"`
	Name      string     `yaml:"name"`
	DependsOn []string   `yaml:"dependsOn"`
	Endpoints []Endpoint `yaml:"-"`

	Routes   []string `yaml:"routes"`
	WsRoutes []string `yaml:"wsRoutes"`
}

// Endpoint is an API endpoint with verb, base path (i.e.: /api/v1 ) and path (i.e.: /apps)
type Endpoint struct {
	Method   string
	BasePath string
	Path     string
}

func NewEndpoint(route routes.Route) Endpoint {
	return newEndpoint("/api/v1", route)
}

func NewWsEndpoint(route routes.Route) Endpoint {
	return newEndpoint("/wapi/v1", route)
}

func newEndpoint(basePath string, route routes.Route) Endpoint {
	return Endpoint{
		Method:   route.Method,
		BasePath: basePath,
		Path:     route.Path,
	}
}

func (e *Endpoint) FullPath() string {
	return e.BasePath + e.Path
}

// InitActions will load the yaml containing the Actions/Routes mapping, and their dependencies
func InitActions() ([]Action, error) {
	actions := []Action{}

	err := yaml.Unmarshal(actionsYAML, &actions)
	if err != nil {
		return actions, errors.Wrap(err, "loading actions from yaml")
	}

	// load map for quick lookup
	for _, action := range actions {
		ActionsMap[action.ID] = action
	}

	// load dependencies routes and wsRoutes
	for i, action := range actions {
		for _, dependencyID := range action.DependsOn {
			// TODO check existence
			dependency, found := ActionsMap[dependencyID]
			if !found {
				return actions, fmt.Errorf("action dependency '%s' not found in ActionsMap [actions.yaml]", dependencyID)
			}
			action = action.Merge(dependency)
		}

		ActionsMap[action.ID] = action
		actions[i] = action
	}

	return actions, nil
}

// IsAllowed check if the action allows the called APIs checking the available endpoints
func (a *Action) IsAllowed(method, fullpath string) bool {
	for _, e := range a.Endpoints {
		// find if the action allows the requested API
		if e.Method == method && e.FullPath() == fullpath {
			return true
		}
	}

	return false
}

// Merge will add the routes and wsRoutes from the dependency into the action
func (a *Action) Merge(dependency Action) Action {
	a.Routes = mergeAndSort(a.Routes, dependency.Routes)
	a.WsRoutes = mergeAndSort(a.WsRoutes, dependency.WsRoutes)
	return *a
}

// mergeAndSort will merge the values from the second array into the first.
// It will remove the duplicates and sort the resulting array.
func mergeAndSort(arr1, arr2 []string) []string {
	arr1 = append(arr1, arr2...)

	// unique map
	m := make(map[string]struct{})
	for _, v := range arr1 {
		m[v] = struct{}{}
	}

	uniqueValues := maps.Keys(m)
	sort.Strings(uniqueValues)

	return uniqueValues
}
