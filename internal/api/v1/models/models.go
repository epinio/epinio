// Package models contains the types (mostly structures) encapsulating
// the API requests and reponses used by the communication between
// epinio client and APIserver.
// Not all of them unfortunately, the simpler ones are coded directly.
// TODO: Give even the most simple requests and responses properly named types.
package models

// ServiceResponse represents the data of a single service instance
type ServiceResponse struct {
	Name      string   `json:"name"`
	BoundApps []string `json:"boundapps"`
}

// ServiceResponseList represents a collection of service instance
type ServiceResponseList []ServiceResponse

// CatalogCreateRequest represents and contains the data needed to
// create a catalog-based service instance
type CatalogCreateRequest struct {
	Name             string `json:"name"`
	Class            string `json:"class"`
	Plan             string `json:"plan"`
	Data             string `json:"data"`
	WaitForProvision bool   `json:"waitforprovision"`
}

// CustomCreateRequest represents and contains the data needed to
// create a custom service instance
type CustomCreateRequest struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

// DeleteRequest represents and contains the data needed to delete a service
type DeleteRequest struct {
	Unbind bool `json:"unbind"`
}

// DeleteResponse represents the server's response to a successful service deletion
type DeleteResponse struct {
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

// ApplicationCreateRequest  represents and contains the data needed to
// create an application
type ApplicationCreateRequest struct {
	Name string `json:"name"`
}

// UpdateAppRequest represents and contains the data needed to update
// an application. Specifically to modify the number of replicas to
// run.
type UpdateAppRequest struct {
	Instances int32 `json:"instances"`
}

// TODO: CreateOrgRequest

// UploadRequest is a multipart form

// UploadResponse represents the server's response to a successful app sources upload
type UploadResponse struct {
	Git *GitRef `json:"git,omitempty"`
}

// StageRequest represents and contains the data needed to stage an application
type StageRequest struct {
	App          AppRef  `json:"app,omitempty"`
	Git          *GitRef `json:"git,omitempty"`
	BuilderImage string  `json:"builderimage,omitempty"`
}

// StageResponse represents the server's response to a successful app staging
type StageResponse struct {
	Stage    StageRef `json:"stage,omitempty"`
	ImageURL string   `json:"image,omitempty"`
}

// DeployRequest represents and contains the data needed to deploy an application
type DeployRequest struct {
	App       AppRef   `json:"app,omitempty"`
	Instances *int32   `json:"instances,omitempty"`
	Stage     StageRef `json:"stage,omitempty"`
	Git       *GitRef  `json:"git,omitempty"`
	ImageURL  string   `json:"image,omitempty"`
}

// DeployResponse represents the server's response to a successful app deployment
type DeployResponse struct {
	Route string `json:"route,omitempty"`
}

// ApplicationDeleteResponse represents the server's response to a successful app deletion
type ApplicationDeleteResponse struct {
	UnboundServices []string `json:"unboundservices"`
}
