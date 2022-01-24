// Package models contains the types (mostly structures) encapsulating
// the API requests and reponses used by the communication between
// epinio client and APIserver.
// Not all of them unfortunately, the simpler ones are coded directly.
// TODO: Give even the most simple requests and responses properly named types.
package models

import "fmt"

type Response struct {
	Status string `json:"status"`
}

var ResponseOK = Response{"ok"}

type Request struct {
}

// ServiceRef references a Service by name and namespace
type ServiceRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// ServiceResponse represents the data of a single service instance
type ServiceResponse struct {
	Meta          ServiceRef          `json:"meta"`
	Configuration ServiceShowResponse `json:"configuration"`
}

// ServiceResponseList represents a collection of service instance
type ServiceResponseList []ServiceResponse

// ServiceCreateRequest represents and contains the data needed to
// create a service instance
type ServiceCreateRequest struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

// ServiceUpdateRequest represents and contains the data needed to
// update a service instance (add/change, and remove keys)
type ServiceUpdateRequest struct {
	Remove []string          `json:"remove,omitempty"`
	Set    map[string]string `json:"edit,omitempty"`
}

// ServiceReplaceRequest represents and contains the data needed to
// replace a service instance
type ServiceReplaceRequest map[string]string

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

// BindResponse represents the server's response to the successful binding of services to
// an application.
type BindResponse struct {
	WasBound []string `json:"wasbound"`
}

// ApplicationManifest represents and contains the data of an application's manifest file,
// plus some auxiliary data never (un)marshaled. Namely, the file's location, and origin
// type tag.
type ApplicationManifest struct {
	ApplicationCreateRequest `yaml:",inline"`
	Self                     string            `yaml:"-"` // Hidden from yaml. The file's location.
	Origin                   ApplicationOrigin `yaml:"origin,omitempty"`
	Staging                  ApplicationStage  `yaml:"staging,omitempty"`
}

// ApplicationStaging is the part of the manifest holding information relevant to staging
// the application's sources. This is, currently, only the reference to the Paketo builder
// image to use.
type ApplicationStage struct {
	Builder string `yaml:"builder,omitempty"`
}

// ApplicationOrigin is the part of the manifest describing the origin of the application
// (sources). At most one of the fields may be specified / not empty.
type ApplicationOrigin struct {
	Kind      int     `json:"-"                   yaml:"-" ` // Hidden from json and yaml. Type tag to simplify struct usage.
	Container string  `json:"container,omitempty" yaml:"container,omitempty" `
	Git       *GitRef `json:"git,omitempty"       yaml:"git,omitempty" `
	Path      string  `json:"path,omitempty"      yaml:"path,omitempty" `
}

// manifest origin codes for `Kind`.
const (
	OriginNone = iota
	OriginPath
	OriginGit
	OriginContainer
)

func (o *ApplicationOrigin) String() string {
	switch o.Kind {
	case OriginPath:
		return o.Path
	case OriginGit:
		if o.Git.Revision == "" {
			return o.Git.URL
		}
		return fmt.Sprintf("%s @ %s", o.Git.URL, o.Git.Revision)
	case OriginContainer:
		return o.Container
	default:
		// Nonthing
	}
	return "<<undefined>>"
}

// ApplicationCreateRequest represents and contains the data needed to
// create an application (at rest), possibly with presets (services)
type ApplicationCreateRequest struct {
	Name          string                   `json:"name"          yaml:"name"`
	Configuration ApplicationUpdateRequest `json:"configuration" yaml:"configuration,omitempty"`
}

// ApplicationUpdateRequest represents and contains the data needed to update
// an application. Specifically to modify the number of replicas to
// run, and the services bound to it.
// Note: Instances is a pointer to give us a nil value separate from
// actual integers, as means of communicating `default`/`no change`.
type ApplicationUpdateRequest struct {
	Instances   *int32         `json:"instances"   yaml:"instances,omitempty"`
	Services    []string       `json:"services"    yaml:"services,omitempty"`
	Environment EnvVariableMap `json:"environment" yaml:"environment,omitempty"`
	Routes      []string       `json:"routes" yaml:"routes,omitempty"`
}

type ImportGitResponse struct {
	BlobUID string `json:"blobuid,omitempty"`
}

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
// This request not only comes with the image to deploy, but also the
// information where the sources of that image came from.
type DeployRequest struct {
	App      AppRef            `json:"app,omitempty"`
	Stage    StageRef          `json:"stage,omitempty"`
	ImageURL string            `json:"image,omitempty"`
	Origin   ApplicationOrigin `json:"origin,omitempty"`
}

// DeployResponse represents the server's response to a successful app deployment
type DeployResponse struct {
	Routes []string `json:"routes,omitempty"`
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
	Username  string            `json:"user"`
	Details   map[string]string `json:"details,omitempty"`
	BoundApps []string          `json:"boundapps"`
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
