package admincmd

import (
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

type ComponentType string
type DeploymentID string
type CheckType string

const (
	YAML ComponentType = "yaml"
	Helm ComponentType = "helm"

	Epinio      DeploymentID = "epinio"
	CertManager DeploymentID = "cert-manager"
	Linkerd     DeploymentID = "linked"
	Kubed       DeploymentID = "kubed"
	// ...

	Pod          CheckType = "pod"
	Loadbalancer CheckType = "loadbalancer"
	CRD          CheckType = "crd"
)

func Load(path string) (*Manifest, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	m := &Manifest{}
	err = yaml.Unmarshal(b, m)
	if err != nil {
		return nil, err
	}

	return m, nil

}

// BuildPlan finds a path through the dag, traversing all nodes using Kahn's algorithm
func BuildPlan(components Components) (Components, error) {
	// L ← Empty list that will contain the sorted elements
	plan := make(Components, 0, len(components))

	// S ← Set of all nodes with no incoming edge
	noedge := map[DeploymentID]bool{}

	// graph has all the edges
	graph := map[DeploymentID]DeploymentID{}
	for _, c := range components {
		if c.Needs == "" {
			continue
		}

		graph[c.ID] = c.Needs
	}

	for _, c := range components {
		if graph[c.ID] == "" {
			noedge[c.ID] = true
		}
	}

	// while S is not empty do
	for len(noedge) > 0 {
		//     remove a node n from S
		var n Component
		for _, c := range components {
			if noedge[c.ID] {
				n = c
				delete(noedge, c.ID)
				break
			}
		}

		//     add n to L
		plan = append(plan, n)

		//     for each node m with an edge e from n to m do
		for m, t := range graph {
			//         remove edge e from the graph
			//         if m has no other incoming edges then
			//             insert m into S
			if t == n.ID {
				delete(graph, m)
				// TODO other edges if needs is an array
				noedge[m] = true
			}
		}
	}

	// if graph has edges then
	//     return error   (graph has at least one cycle)
	// else
	//     return L   (a topologically sorted order)

	if len(graph) > 0 {
		return plan, fmt.Errorf("cycle: has edges %v", graph)
	}

	return plan, nil
}

func Runner(plan Components) error {
	state := map[DeploymentID]bool{}
	running := map[DeploymentID]bool{}
	for _, c := range plan {
		state[c.ID] = false
		running[c.ID] = false
	}

	var lock = &sync.RWMutex{}
	for !allDone(lock, state) {
		for _, c := range plan {
			c := c
			lock.RLock()
			if state[c.ID] {
				//fmt.Printf("skip done: %s\n", c.ID)
				lock.RUnlock()
				continue
			}
			if running[c.ID] {
				//fmt.Printf("skip running: %s\n", c.ID)
				lock.RUnlock()
				continue
			}
			if c.Needs != "" && !state[c.Needs] {
				//fmt.Printf("skip '%s' for deps: %s (r:%v, d:%v)\n", c.ID, c.Needs, running[c.Needs], state[c.Needs])
				lock.RUnlock()
				continue
			}
			lock.RUnlock()

			//fmt.Printf("did not skip: %s\n", c.ID)
			lock.Lock()
			running[c.ID] = true
			lock.Unlock()

			go func() {
				fmt.Printf("starting %s\n", c.ID)

				// TODO run the actual deployment
				time.Sleep(3 * time.Second)

				// TODO run the wait for

				lock.Lock()
				state[c.ID] = true
				lock.Unlock()

				fmt.Printf("finished %s\n", c.ID)
			}()
		}
	}

	return nil
}

func allDone(lock *sync.RWMutex, s map[DeploymentID]bool) bool {
	lock.RLock()
	defer lock.RUnlock()
	for _, done := range s {
		if !done {
			return false
		}
	}
	return true
}

type Manifest struct {
	// Values specifies user inputs
	// TODO needed?
	Values map[string]string

	// Generate everything the installer should build first
	Generate []interface{}

	// Components are known to Epinio, this describes how to install them
	Components Components
}

type Component struct {
	// ID identifies the component, e.g. 'linkerd'
	ID DeploymentID `json:"id" yaml:"id"`

	// Type is 'helm' or 'yaml'
	Type ComponentType `json:"type" yaml:"type"`

	// Wait for the helm chart's install
	Wait bool `json:"wait,omitempty" yaml:"wait"`

	// WaitComplete is a list of checks to make sure the component is complete
	WaitComplete []Check `json:"wait_complete,omitempty" yaml:"waitComplete"`

	// Source for the component (was repo/path/..)
	Source Source

	// Values to be used when installing this component
	Values []Value

	// Needs is used to build a DAG of components for the installation order
	Needs DeploymentID
	//Needs []string
}

type Components []Component

func (cs Components) IDs() []DeploymentID {
	ids := make([]DeploymentID, 0, len(cs))
	for _, c := range cs {
		ids = append(ids, c.ID)
	}
	return ids
}

type Check struct {
	// Type is 'pod', 'loadbalancer' or 'crd', the check is implemented in code
	Type     CheckType `json:"type" yaml:"type"`
	Selector string    `json:"selector,omitempty" yaml:"selector"`
}

// not sure
type Source struct {
	Name    string `yaml:"name"`
	URL     string `yaml:"url"`
	Chart   string `yaml:"chart"`
	Version string `yaml:"version"`
	Path    string `yaml:"path"`
}

type Value struct {
	Name  string
	Value string
}
