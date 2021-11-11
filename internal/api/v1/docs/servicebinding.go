package docs

//go:generate swagger generate spec

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// Service Bindings

// swagger:route POST   /namespaces/{Namespace}/applications/{App}/servicebindings svc-binding ServiceBindingCreate
// Create service binding between `App` in `Namespace`, and the posted services, also in `Namespace`.
// responses:
//   200: ServiceBindResponse

// swagger:parameters ServiceBindingCreate
type ServiceBindingCreateParams struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: body
	Body models.BindRequest
}

// swagger:response ServiceBindResponse
type ServiceBindResponse struct {
	// in: body
	Body models.BindResponse
}

// swagger:route DELETE /namespaces/{Namespace}/applications/{App}/servicebindings/{Service} svc-binding ServiceBindingDelete
// Remove service binding between `App` and `Service` in `Namespace`.
// responses:
//   200: ServiceUnbindReponse

// swagger:parameters ServiceBindingDelete
type ServiceBindingDeleteParams struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: path
	Service string
}

// swagger:response ServiceUnbindReponse
type ServiceUnbindReponse struct {
	// in:body
	Body models.Response
}
