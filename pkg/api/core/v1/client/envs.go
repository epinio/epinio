package client

import (
	"encoding/json"

	"github.com/pkg/errors"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// EnvList returns a map of all env vars for an app
func (c *Client) EnvList(org string, appName string) (models.EnvVariableMap, error) {
	var resp models.EnvVariableMap

	data, err := c.get(api.Routes.Path("EnvList", org, appName))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}

// EnvSet set env vars for an app
func (c *Client) EnvSet(req models.EnvVariableMap, org string, appName string) (models.Response, error) {
	resp := models.Response{}

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("EnvSet", org, appName), string(b))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	return resp, nil
}

// EnvShow shows an env variable
func (c *Client) EnvShow(org string, appName string, envName string) (models.EnvVariable, error) {
	resp := models.EnvVariable{}

	data, err := c.get(api.Routes.Path("EnvShow", org, appName, envName))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}

// EnvUnset removes an env var
func (c *Client) EnvUnset(org string, appName string, envName string) (models.Response, error) {
	resp := models.Response{}

	data, err := c.delete(api.Routes.Path("EnvUnset", org, appName, envName))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}

// EnvMatch returns all env vars matching the prefix
func (c *Client) EnvMatch(org string, appName string, prefix string) (models.EnvMatchResponse, error) {
	resp := models.EnvMatchResponse{}

	data, err := c.get(api.Routes.Path("EnvMatch", org, appName, prefix))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}
