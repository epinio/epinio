package client

import (
	"encoding/json"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

func (c *Client) ServiceCatalog() (*models.ServiceCatalogResponse, error) {
	data, err := c.get(api.Routes.Path("ServiceCatalog"))
	if err != nil {
		return nil, err
	}

	var resp models.ServiceCatalogResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return &resp, nil
}

func (c *Client) ServiceCatalogShow(serviceName string) (*models.ServiceCatalogShowResponse, error) {
	data, err := c.get(api.Routes.Path("ServiceCatalogShow", serviceName))
	if err != nil {
		return nil, err
	}

	var resp models.ServiceCatalogShowResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return &resp, nil
}

func (c *Client) ServiceCreate(req *models.ServiceCreateRequest, namespace string) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}

	_, err = c.post(api.Routes.Path("ServiceCreate", namespace), string(b))
	return err
}
