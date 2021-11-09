package docs

import (
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

//go:generate swagger generate spec

// swagger:route GET /namespaces namespace Namespaces
// Return map of all controlled namespaces.
// responses:
//   200: NamespacesResponse

// swagger:response NamespacesResponse
type NamespacesResponse struct {
	// in: body
	Body models.NamespaceList
}

// swagger:route POST /namespaces/{namespace} namespace NamespaceCreate
// Create a new named `namespace`.
// responses:
//   200: NamespaceCreateResponse

// swagger:parameters NamespaceCreate
type NamespaceCreateParam struct {
	Namespace string
	// in: body
	Body models.NamespaceCreateRequest
}

// swagger:response NamespaceCreateResponse
type NamespaceCreateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route DELETE /namespaces/{namespace} namespace NamespaceDelete
// Delete the named `namespace`.
// responses:
//   200: NamespaceDeleteResponse

// swagger:parameters NamespaceCreate
type NamespaceDeleteParam struct {
	Namespace string
}

// swagger:response NamespaceDeleteResponse
type NamespaceDeleteResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /namespaces/{namespace} namespace NamespaceShow
// Return details of the named `namespace`.
// responses:
//   200: NamespaceShowResponse

// swagger:parameters NamespaceShow
type NamespaceShowParam struct {
	Namespace string
}

// swagger:response NamespaceShowResponse
type NamesapceShowResponse struct {
	// in: body
	Body models.Namespace
}

// swagger:route GET /namespaces/{pattern} namespace NamespaceMatch
// Return map of all controlled namespaces matching the prefix `pattern`.
// responses:
//   200: NamespaceMatchResponse

// swagger:parameters NamespaceMatch
type NamespaceMatchParam struct {
	Pattern string
}

// swagger:response NamespaceMatchResponse
type NamesapceMatchResponse struct {
	// in: body
	Body models.NamespacesMatchResponse
}

// swagger:route GET /namespaces namespace NamespaceMatch0
// Return map of all namespaces.
// responses:
//   200: NamespaceMatchResponse
