package client

import (
	"encoding/json"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// NamespaceCreate creates a namespace
func (c *Client) NamespaceCreate(req models.NamespaceCreateRequest) (models.Response, error) {
	var resp models.Response

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("Namespaces"), string(b))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// NamespaceDelete deletes a namespace
func (c *Client) NamespaceDelete(namespace string) (models.Response, error) {
	resp := models.Response{}

	data, err := c.delete(api.Routes.Path("NamespaceDelete", namespace))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// NamespaceShow shows a namespace
func (c *Client) NamespaceShow(namespace string) (models.Namespace, error) {
	resp := models.Namespace{}

	data, err := c.get(api.Routes.Path("NamespaceShow", namespace))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// NamespacesMatch returns all matching namespaces for the prefix
func (c *Client) NamespacesMatch(prefix string) (models.NamespacesMatchResponse, error) {
	resp := models.NamespacesMatchResponse{}

	data, err := c.get(api.Routes.Path("NamespacesMatch", prefix))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// Namespaces returns a list of namespaces
func (c *Client) Namespaces() (models.NamespaceList, error) {
	resp := models.NamespaceList{}

	data, err := c.get(api.Routes.Path("Namespaces"))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}
