package client

import (
	"encoding/json"

	"github.com/pkg/errors"

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

// ChartCreate creates a new application chart
func (c *Client) ChartCreate(request models.ChartCreateRequest) (models.Response, error) {
	resp := models.Response{}

	b, err := json.Marshal(request)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("ChartCreate"), string(b))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
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

// ChartDelete removes a named application chart
func (c *Client) ChartDelete(name string) (models.Response, error) {
	resp := models.Response{}

	data, err := c.delete(api.Routes.Path("ChartDelete", name))
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
