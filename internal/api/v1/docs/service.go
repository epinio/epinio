package docs

//go:generate swagger generate spec

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// Services

// swagger:route DELETE /namespaces/{Namespace}/services/{Service} service ServiceDelete
// Delete the named `Service` in the `Namespace`.
// responses:
//   200: ServiceDeleteResponse

// swagger:parameters ServiceDelete
type ServiceDeleteParam struct {
	// in: path
	Namespace string
	// in: path
	Service string
}

// swagger:response ServiceDeleteResponse
type ServiceDeleteResponse struct {
	// in: body
	Body models.ServiceDeleteResponse
}

// swagger:route GET /namespaces/{Namespace}/serviceapps service ServiceApps
// Return map from services in the `Namespace`, to the apps in the same.
// responses:
//   200: ServiceAppsResponse

// swagger:parameters ServiceApps
type ServiceAppsParam struct {
	// in: path
	Namespace string
}

// swagger:response ServiceAppsResponse
type ServiceAppsResponse struct {
	// in: body
	Body map[string]models.AppList
}

// swagger:route GET /namespaces/{Namespace}/services service Services
// Return list of services in the `Namespace`.
// responses:
//   200: ServicesResponse

// swagger:parameters Services
type ServicesParam struct {
	// in: path
	Namespace string
}

// swagger:response ServicesResponse
type ServicesResponse struct {
	// in: body
	Body models.ServiceResponseList
}

// swagger:route GET /namespaces/{Namespace}/services/{Service} service ServiceShow
// Return details of the named `Service` in the `Namespace`.
// responses:
//   200: ServiceShowResponse

// swagger:parameters ServiceShow
type ServiceShowParam struct {
	// in: path
	Namespace string
	// in: path
	Service string
}

// swagger:response ServiceShowResponse
type ServiceShowResponse struct {
	// in: body
	Body models.ServiceShowResponse
}

// swagger:route POST /namespaces/{Namespace}/services service ServiceCreate
// Create the posted new service in the `Namespace`.
// responses:
//   200: ServiceCreateResponse

// swagger:parameters ServiceCreate
type ServiceCreateParam struct {
	// in: path
	Namespace string
	// in: body
	Service models.ServiceCreateRequest
}

// swagger:route PATCH /namespaces/{Namespace}/services/{Service} service ServiceUpdate
// Update the named `Service` in the `Namespace` as per the instructions in the body
// responses:
//   200: ServiceUpdateResponse

// swagger:parameters ServiceUpdate
type ServiceUpdateParam struct {
	// in: path
	Namespace string
	// in: path
	Service string
	// in: body
	Body models.ServiceUpdateRequest
}

// swagger:route PUT /namespaces/{Namespace}/services/{Service} service ServiceReplace
// Replace the named `Service` in the `Namespace` as per the instructions in the body
// responses:
//   200: ServiceReplaceResponse

// swagger:parameters ServiceReplace
type ServiceReplaceParam struct {
	// in: path
	Namespace string
	// in: path
	Service string
	// in: body
	Body models.ServiceReplaceRequest
}

// swagger:response ServiceUpdateResponse
type ServiceUpdateResponse struct {
	// in: body
	Body models.Response
}

// swagger:response ServiceCreateResponse
type ServiceCreateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /services service AllServices
// Return list of services in all namespaces.
// responses:
//   200: ServicesResponse

// swagger:parameters AllServices
type ServiceAllServicesParam struct{}

// response: See Services.
