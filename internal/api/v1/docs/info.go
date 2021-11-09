package docs

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// Info

// swagger:route /info info Info
// Return server system information
// responses:
//   200: ServiceInfoResponse

// swagger:response InfoResponse
type InfoResponse struct {
	// in: body
	Body models.InfoResponse
}
