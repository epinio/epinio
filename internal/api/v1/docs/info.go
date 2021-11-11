package docs

//go:generate swagger generate spec

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// Info

// swagger:route GET /info info Info
// Return server system information
// responses:
//   200: InfoResponse

// swagger:response InfoResponse
type InfoResponse struct {
	// in: body
	Body models.InfoResponse
}
