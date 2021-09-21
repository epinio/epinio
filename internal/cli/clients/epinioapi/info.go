package epinioapi

import (
	"encoding/json"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/models"
)

// Info returns information about Epinio and its components
func (c *Client) Info() (models.InfoResponse, error) {
	var resp models.InfoResponse

	data, err := c.get(api.Routes.Path("Info"))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}
