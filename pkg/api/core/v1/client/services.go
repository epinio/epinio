package client

import (
	"encoding/json"

	"github.com/pkg/errors"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

func (c *Client) ServiceCatalog() (models.CatalogServices, error) {
	data, err := c.get(api.Routes.Path("ServiceCatalog"))
	if err != nil {
		return nil, err
	}

	var resp models.CatalogServices
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

func (c *Client) ServiceCatalogShow(serviceName string) (*models.CatalogService, error) {
	data, err := c.get(api.Routes.Path("ServiceCatalogShow", serviceName))
	if err != nil {
		return nil, err
	}

	var resp models.CatalogService
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return &resp, nil
}

func (c *Client) AllServices() (models.ServiceList, error) {
	data, err := c.get(api.Routes.Path("AllServices"))
	if err != nil {
		return nil, err
	}

	var resp models.ServiceList
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, err
}

func (c *Client) ServiceCreate(req *models.ServiceCreateRequest, namespace string) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}

	_, err = c.post(api.Routes.Path("ServiceCreate", namespace), string(b))
	return err
}

func (c *Client) ServiceShow(req *models.ServiceShowRequest, namespace string) (*models.Service, error) {
	data, err := c.get(api.Routes.Path("ServiceShow", namespace, req.Name))
	if err != nil {
		return nil, err
	}

	var resp models.Service
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return &resp, nil
}

func (c *Client) ServiceDelete(req models.ServiceDeleteRequest, namespace string, name string, f ErrorFunc) (models.ServiceDeleteResponse, error) {

	resp := models.ServiceDeleteResponse{}

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.doWithCustomErrorHandling(
		api.Routes.Path("ServiceDelete", namespace, name),
		"DELETE", string(b), f)
	if err != nil {
		if err.Error() != "Bad Request" {
			return resp, err
		}
		return resp, nil
	}

	if len(data) > 0 {
		if err := json.Unmarshal(data, &resp); err != nil {
			return resp, errors.Wrap(err, "response body is not JSON")
		}
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

func (c *Client) ServiceBind(req *models.ServiceBindRequest, namespace, name string) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}

	_, err = c.post(api.Routes.Path("ServiceBind", namespace, name), string(b))
	return err
}

func (c *Client) ServiceUnbind(req *models.ServiceUnbindRequest, namespace, name string) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}

	_, err = c.post(api.Routes.Path("ServiceUnbind", namespace, name), string(b))
	return err
}

func (c *Client) ServiceList(namespace string) (models.ServiceList, error) {
	data, err := c.get(api.Routes.Path("ServiceList", namespace))
	if err != nil {
		return nil, err
	}

	var resp models.ServiceList
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, err
}

// ServiceApps lists a map from services to bound apps, for the namespace
func (c *Client) ServiceApps(namespace string) (models.ServiceAppsResponse, error) {
	resp := models.ServiceAppsResponse{}

	data, err := c.get(api.Routes.Path("ServiceApps", namespace))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}
