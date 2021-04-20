package models

// The structures in this package encapsulate the requested and
// response data used by the communication between cli and api server.

type ServiceResponse struct {
	Name      string   `json:"name"`
	BoundApps []string `json:"boundapps"`
}

type ServiceResponseList []ServiceResponse

type CatalogCreateRequest struct {
	Name             string            `json:"name"`
	Class            string            `json:"class"`
	Plan             string            `json:"plan"`
	Data             map[string]string `json:"data"`
	WaitForProvision bool              `json:"waitforprovision"`
}

type CustomCreateRequest struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

type DeleteRequest struct {
	Unbind bool `json:"unbind"`
}

type DeleteResponse struct {
	BoundApps []string `json:"boundapps"`
}

type BindRequest struct {
	Name string `json:"name"`
}

// TODO: CreateOrgRequest
