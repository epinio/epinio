// Package models contains the types (mostly structures) encapsulating
// the API requests and reponses used by the communication between
// epinio client and APIserver.
// Not all of them unfortunately, the simpler ones are coded directly.
// TODO: Give even the most simple requests and responses properly named types.
package models

type Response struct {
	Status string `json:"status"`
}

var ResponseOK = Response{"ok"}

type Request struct {
}

// ServiceResponse represents the data of a single service instance
type ServiceResponse struct {
	Name      string   `json:"name"`
	BoundApps []string `json:"boundapps"`
}

// ServiceResponseList represents a collection of service instance
type ServiceResponseList []ServiceResponse

// ServiceCreateRequest represents and contains the data needed to
// create a service instance
type ServiceCreateRequest struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

// ServiceDeleteRequest represents and contains the data needed to delete a service
type ServiceDeleteRequest struct {
	Unbind bool `json:"unbind"`
}

// ServiceDeleteResponse represents the server's response to a successful service deletion
type ServiceDeleteResponse struct {
	BoundApps []string `json:"boundapps"`
}

// BindRequest represents and contains the data needed to bind services to an application.
type BindRequest struct {
	Names []string `json:"names"`
}

// BindResponse represents the server's response to successful binding of services to an app
type BindResponse struct {
	WasBound []string `json:"wasbound"`
}

// ApplicationCreateRequest represents and contains the data needed to
// create an application (at rest), possibly with presets (services)
type ApplicationCreateRequest struct {
	Name          string                   `json:"name"`
	Configuration ApplicationUpdateRequest `json:"configuration"`
}

// ApplicationUpdateRequest represents and contains the data needed to update
// an application. Specifically to modify the number of replicas to
// run, and the services bound to it.
// Note: Instances is a pointer to give us a nil value separate from
// actual integers, as means of communicating `default`/`no change`.

type ApplicationUpdateRequest struct {
	Instances   *int32          `json:"instances"`
	Services    []string        `json:"services"`
	Environment EnvVariableList `json:"environment"`
}

type ImportGitResponse struct {
	BlobUID string `json:"blobuid,omitempty"`
}

// TODO: CreateOrgRequest

// UploadRequest is a multipart form

// UploadResponse represents the server's response to a successful app sources upload
type UploadResponse struct {
	BlobUID string `json:"blobuid,omitempty"`
}

// StageRequest represents and contains the data needed to stage an application
type StageRequest struct {
	App          AppRef `json:"app,omitempty"`
	BlobUID      string `json:"blobuid,omitempty"`
	BuilderImage string `json:"builderimage,omitempty"`
}

// StageResponse represents the server's response to a successful app staging
type StageResponse struct {
	Stage    StageRef `json:"stage,omitempty"`
	ImageURL string   `json:"image,omitempty"`
}

// DeployRequest represents and contains the data needed to deploy an application
// Note that the overall application configuration (instances, services, EVs) is
// already known server side, through AppCreate/AppUpdate requests.
type DeployRequest struct {
	App      AppRef   `json:"app,omitempty"`
	Stage    StageRef `json:"stage,omitempty"`
	ImageURL string   `json:"image,omitempty"`
}

// DeployResponse represents the server's response to a successful app deployment
type DeployResponse struct {
	Route string `json:"route,omitempty"`
}

// ApplicationDeleteResponse represents the server's response to a successful app deletion
type ApplicationDeleteResponse struct {
	UnboundServices []string `json:"unboundservices"`
}

// EnvMatchResponse contains the list of names for matching envs
type EnvMatchResponse struct {
	Names []string `json:"names,omitempty"`
}

// ServiceShowResponse contains details about a service
type ServiceShowResponse struct {
	Username string            `json:"user"`
	Details  map[string]string `json:"details,omitempty"`
}

// InfoResponse contains information about Epinio and its components
type InfoResponse struct {
	Version     string `json:"version,omitempty"`
	KubeVersion string `json:"kube_version,omitempty"`
	Platform    string `json:"platform,omitempty"`
}

// NamespaceCreateRequest contains the name of the namespace that should be created
type NamespaceCreateRequest struct {
	Name string `json:"name,omitempty"`
}

// NamespacesMatchResponse contains the list of names for matching namespaces
type NamespacesMatchResponse struct {
	Names []string `json:"names,omitempty"`
}

// ServiceAppsResponse returns a list of apps per service
type ServiceAppsResponse struct {
	AppsOf map[string]AppList `json:"apps_of,omitempty"`
}
