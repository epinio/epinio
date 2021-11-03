package models

import (
	"github.com/epinio/epinio/internal/names"
)

const (
	EpinioStageIDLabel      = "epinio.suse.org/stage-id"
	EpinioStageBlobUIDLabel = "epinio.suse.org/blob-uid"

	ApplicationCreated = "created"
	ApplicationStaging = "staging"
	ApplicationRunning = "running"
	ApplicationError   = "error"
)

type ApplicationStatus string

type GitRef struct {
	Revision string `json:"revision" yaml:"revision,omitempty"`
	URL      string `json:"url"      yaml:"url"`
}

// App has all the application's properties, for at rest (Configuration), and active (Workload).
// The main structure has identifying information.
// It is used in the CLI and API responses.
// If an error is hit while constructing the app object, the Error attribute
// will be set to that.
type App struct {
	Meta          AppRef                   `json:"meta"`
	Configuration ApplicationUpdateRequest `json:"configuration"`
	Workload      *AppDeployment           `json:"deployment,omitempty"`
	Status        ApplicationStatus        `json:"status"`
	StatusMessage string                   `json:"statusmessage"`
}

// AppDeployment contains all the information specific to an active
// application, i.e. one with a deployment in the cluster.
type AppDeployment struct {
	// TODO: Readiness and Liveness fields?
	Active          bool   `json:"active,omitempty"` // app is > 0 replicas
	CreatedAt       string `json:"createdAt,omitempty"`
	Restarts        int32  `json:"restarts"`
	MemoryBytes     int64  `json:"memoryBytes"`
	MilliCPUs       int64  `json:"millicpus"`
	DesiredReplicas int32  `json:"desiredreplicas"`
	ReadyReplicas   int32  `json:"readyreplicas"`
	Username        string `json:"username,omitempty"` // app creator
	StageID         string `json:"stage_id,omitempty"` // tekton staging id
	Status          string `json:"status,omitempty"`   // app replica status
	Route           string `json:"route,omitempty"`    // app route
}

// NewApp returns a new app for name and org
func NewApp(name string, org string) *App {
	return &App{
		Meta: AppRef{
			Name: name,
			Org:  org,
		},
	}
}

// AppRef returns a reference to the app (name, org)
func (a *App) AppRef() AppRef {
	return a.Meta
}

// AppList is a collection of app references
type AppList []App

// Implement the Sort interface for application slices

// Len (Sort interface) returns the length of the AppList
func (al AppList) Len() int {
	return len(al)
}

// Swap (Sort interface) exchanges the contents of specified indices
// in the AppList
func (al AppList) Swap(i, j int) {
	al[i], al[j] = al[j], al[i]
}

// Less (Sort interface) compares the contents of the specified
// indices in the AppList and returns true if the condition holds, and
// else false.
func (al AppList) Less(i, j int) bool {
	return (al[i].Meta.Org < al[j].Meta.Org) ||
		((al[i].Meta.Org == al[j].Meta.Org) &&
			(al[i].Meta.Name < al[j].Meta.Name))
}

// AppRef references an App by name and org
type AppRef struct {
	Name string `json:"name"`
	Org  string `json:"namespace"` // TODO: Rename to Namespace
}

// NewAppRef returns a new reference to an app
func NewAppRef(name string, org string) AppRef {
	return AppRef{Name: name, Org: org}
}

// App returns a fresh app model for the reference
func (ar *AppRef) App() *App {
	return NewApp(ar.Name, ar.Org)
}

// MakeEnvSecretName returns the name of the kube secret holding the
// environment variables of the referenced application
func (ar *AppRef) MakeEnvSecretName() string {
	// TODO: This needs tests for env operations on an app with a long name
	return names.GenerateResourceName(ar.Name + "-env")
}

// MakeServiceSecretName returns the name of the kube secret holding the
// bound services of the referenced application
func (ar *AppRef) MakeServiceSecretName() string {
	// TODO: This needs tests for service operations on an app with a long name
	return names.GenerateResourceName(ar.Name + "-svc")
}

// MakeScaleSecretName returns the name of the kube secret holding the number
// of desired instances for referenced application
func (ar *AppRef) MakeScaleSecretName() string {
	// TODO: This needs tests for service operations on an app with a long name
	return names.GenerateResourceName(ar.Name + "-scale")
}

// MakePVCName returns the name of the kube pvc to use with/for the referenced application.
func (ar *AppRef) MakePVCName() string {
	return names.GenerateResourceName(ar.Org, ar.Name)
}

// StageRef references a tekton staging run by ID, currently randomly generated
// for each POST to the staging endpoint
type StageRef struct {
	ID string `json:"id,omitempty"`
}

// NewStage returns a new reference to a staging run
func NewStage(id string) StageRef {
	return StageRef{id}
}

// ImageRef references an upload
type ImageRef struct {
	ID string `json:"id,omitempty"`
}

// NewImage returns a new image ref for the given ID
func NewImage(id string) ImageRef {
	return ImageRef{id}
}
