package docs

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// Services

// swagger:route DELETE /namespaces/{namespace}/services/{service} service ServiceDelete
// Delete the named `service` in the `namespace`.
// responses:
//   200: ServiceDeleteResponse

// swagger:parameters ServiceDelete
type ServiceDeleteParam struct {
	Namespace string
	Service   string
}

// swagger:response ServiceDeleteResponse
type ServiceDeleteResponse struct {
	// in: body
	Body models.ServiceDeleteResponse
}

// swagger:route GET /namespaces/{namespace}/serviceapps service ServiceApps
// Return map from services in the `namespace`, to the apps in the same.
// responses:
//   200: ServiceAppsResponse

// swagger:parameters ServiceApps
type ServiceAppsParam struct {
	Namespace string
}

// swagger:response ServiceAppsResponse
type ServiceAppsResponse struct {
	// in: body
	Body map[string]models.AppList
}

// swagger:route GET /namespaces/{namespace}/services service Services
// Return list of services in the `namespace`.
// responses:
//   200: ServicesResponse

// swagger:parameters Services
type ServicesParam struct {
	Namespace string
}

// swagger:response ServicesResponse
type ServicesResponse struct {
	// in: body
	Body models.ServiceResponseList
}

// swagger:route GET /namespaces/{namespace}/services/{service} service ServiceShow
// Return details of the named `service` in the `namespace`.
// responses:
//   200: ServiceShowResponse

// swagger:parameters ServiceShow
type ServiceShowParam struct {
	Namespace string
	Service   string
}

// swagger:response ServiceShowResponse
type ServiceShowResponse struct {
	// in: body
	Body models.ServiceShowResponse
}

// swagger:route POST /namespaces/{namespace}/services service ServiceCreate
// Create the posted new service in the `namespace`.
// responses:
//   200: ServiceCreateResponse

// swagger:parameters ServiceCreate
type ServiceCreateParam struct {
	Namespace string
	// in: body
	Service models.ServiceCreateRequest
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
