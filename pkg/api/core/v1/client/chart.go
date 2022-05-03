package client

import (
	"encoding/json"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// ChartList returns a list of all known application charts
func (c *Client) ChartList() ([]models.AppChart, error) {
	var resp []models.AppChart

	data, err := c.get(api.Routes.Path("ChartList"))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ChartShow returns a named application chart
func (c *Client) ChartShow(name string) (models.AppChart, error) {
	resp := models.AppChart{}

	data, err := c.get(api.Routes.Path("ChartShow", name))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ChartMatch returns all application charts whose name matches the prefix
func (c *Client) ChartMatch(prefix string) (models.ChartMatchResponse, error) {
	resp := models.ChartMatchResponse{}

	data, err := c.get(api.Routes.Path("ChartMatch", prefix))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}
