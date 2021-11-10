package docs

//go:generate swagger generate spec

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// Service Bindings

// swagger:route POST   /namespaces/{namespace}/applications/{app}/servicebindings svc-binding ServiceBindingCreate
// Create service binding between `app` in `namespace`, and the posted services, also in `namespace`.
// responses:
//   200: ServiceBindResponse

// swagger:parameters ServiceBindingCreate
type ServiceBindingCreateParams struct {
	Namespace string
	App       string
	// in: body
	Body models.BindRequest
}

// swagger:response ServiceBindResponse
type ServiceBindResponse struct {
	// in: body
	Body models.BindResponse
}

// swagger:route DELETE /namespaces/{namespace}/applications/{app}/servicebindings/{service} svc-binding ServiceBindingDelete
// Remove service binding between `app` and `service` in `namespace`.
// responses:
//   200: ServiceUnbindReponse

// swagger:parameters ServiceBindingDelete
type ServiceBindingDeleteParams struct {
	Namespace string
	App       string
	Service   string
}

// swagger:response ServiceUnbindReponse
type ServiceUnbindReponse struct {
	// in:body
	Body models.Response
}
