package admincmd

import (
	"io/ioutil"

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

type Manifest struct {
	// Values specifies user inputs
	// TODO needed?
	Values map[string]string

	// Generate everything the installer should build first
	Generate []interface{}

	// Components are known to Epinio, this describes how to install them
	Components []Component
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
	Needs string
	//Needs []string
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
