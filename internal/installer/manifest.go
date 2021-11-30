package installer

import (
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

type ComponentType string
type DeploymentID string
type ActionType string
type ValueType string

const (
	YAML      ComponentType = "yaml"
	Helm      ComponentType = "helm"
	Namespace ComponentType = "namespace"

	Job          ActionType = "job"
	Pod          ActionType = "pod"
	Loadbalancer ActionType = "loadbalancer"
	CRD          ActionType = "crd"

	Label      ValueType = "label"
	Annotation ValueType = "annotation"
)

type Manifest struct {
	// Components are known to Epinio, this describes how to install them
	Components Components
}

type Component struct {
	// ID identifies the component, e.g. 'linkerd'
	ID DeploymentID `json:"id" yaml:"id"`

	// Namespace the component is supposed to be in
	Namespace string `json:"namespace" yaml:"namespace"`

	// Type is 'helm' or 'yaml'
	Type ComponentType `json:"type" yaml:"type"`

	// PreDeploy checks make sure the component can be installed
	PreDeploy []ComponentAction `json:"pre_deploy_check,omitempty" yaml:"preDeploy"`

	// WaitComplete is a list of checks to make sure the component is complete
	WaitComplete []ComponentAction `json:"wait_complete,omitempty" yaml:"waitComplete"`

	// Source for the component (was repo/path/..)
	Source Source

	// Values to be used when installing this component
	Values Values

	// Needs is used to build a DAG of components for the installation order
	Needs DeploymentID
}

func (c Component) String() string {
	return string(c.ID)
}

type Components []Component

func (cs Components) IDs() []DeploymentID {
	ids := make([]DeploymentID, 0, len(cs))
	for _, c := range cs {
		ids = append(ids, c.ID)
	}
	return ids
}

func (cs Components) String() string {
	ids := make([]string, 0, len(cs))
	for _, c := range cs {
		ids = append(ids, string(c.ID))
	}
	return strings.Join(ids, ", ")
}

type Values []Value

func (vals Values) ToMap() map[string]string {
	m := map[string]string{}
	for _, v := range vals {
		m[v.Name] = v.Value
	}
	return m
}

type ComponentAction struct {
	// Type is 'pod', 'loadbalancer' or 'crd', the check is implemented in code
	Type      ActionType `json:"type" yaml:"type"`
	Selector  string     `json:"selector,omitempty" yaml:"selector"`
	Namespace string     `json:"namespace" yaml:"namespace"`
}

// Source describes the resource to be installed
// Helm:
// By `Path` to a packaged chart: helm install mynginx ./nginx-1.2.3.tgz
// By `Path` to an unpacked chart directory: helm install mynginx ./nginx
// By absolute `URL`: helm install mynginx https://example.com/charts/nginx-1.2.3.tgz
// By `Chart` reference and repo `URL` and optionally `Version`: helm install --repo https://example.com/charts/ --version 0.1.2 mynginx nginx
type Source struct {
	// Name is the name of the helm release
	Name string `yaml:"name"`

	// Chart is the name of the helm chart, needs a URL for the repo
	Chart   string `yaml:"chart"`
	Path    string `yaml:"path"`
	URL     string `yaml:"url"`
	Version string `yaml:"version"`
}

func (s Source) IsPath() bool {
	return s.Path != "" && s.Chart == "" && s.URL == ""
}

func (s Source) IsURL() bool {
	return s.Path == "" && s.Chart == "" && s.URL != ""
}

func (s Source) IsHelmRef() bool {
	return s.Path == "" && s.Chart != "" && s.URL != ""
}

type Value struct {
	Name  string
	Value string
	Type  ValueType
}

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
