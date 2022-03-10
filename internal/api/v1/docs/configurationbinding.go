package docs

//go:generate swagger generate spec

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// Configuration Bindings

// swagger:route POST   /namespaces/{Namespace}/applications/{App}/configurationbindings svc-binding ConfigurationBindingCreate
// Create configuration binding between `App` in `Namespace`, and the posted configurations, also in `Namespace`.
// responses:
//   200: ConfigurationBindResponse

// swagger:parameters ConfigurationBindingCreate
type ConfigurationBindingCreateParams struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: body
	Body models.BindRequest
}

// swagger:response ConfigurationBindResponse
type ConfigurationBindResponse struct {
	// in: body
	Body models.BindResponse
}

// swagger:route DELETE /namespaces/{Namespace}/applications/{App}/configurationbindings/{Configuration} svc-binding ConfigurationBindingDelete
// Remove configuration binding between `App` and `Configuration` in `Namespace`.
// responses:
//   200: ConfigurationUnbindReponse

// swagger:parameters ConfigurationBindingDelete
type ConfigurationBindingDeleteParams struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: path
	Configuration string
}

// swagger:response ConfigurationUnbindReponse
type ConfigurationUnbindReponse struct {
	// in:body
	Body models.Response
}
