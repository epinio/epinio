package client

import (
	"encoding/json"

	"github.com/pkg/errors"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Services returns a list of services
func (c *Client) Services(org string) (models.ServiceResponseList, error) {
	resp := models.ServiceResponseList{}

	data, err := c.get(api.Routes.Path("Services", org))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ServiceBindingCreate creates a binding from an app to a serviceclass
func (c *Client) ServiceBindingCreate(req models.BindRequest, org string, appName string) (models.BindResponse, error) {
	resp := models.BindResponse{}

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("ServiceBindingCreate", org, appName), string(b))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ServiceBindingDelete deletes a binding from an app to a serviceclass
func (c *Client) ServiceBindingDelete(org string, appName string, serviceName string) (models.Response, error) {
	resp := models.Response{}

	data, err := c.delete(api.Routes.Path("ServiceBindingDelete", org, appName, serviceName))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ServiceDelete deletes a service
func (c *Client) ServiceDelete(req models.ServiceDeleteRequest, org string, name string, f errorFunc) (models.ServiceDeleteResponse, error) {
	resp := models.ServiceDeleteResponse{}

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.doWithCustomErrorHandling(
		api.Routes.Path("ServiceDelete", org, name),
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

// ServiceCreate creates a service by invoking the associated API endpoint
func (c *Client) ServiceCreate(req models.ServiceCreateRequest, org string) (models.Response, error) {
	resp := models.Response{}

	c.log.V(5).WithValues("request", req, "org", org).Info("requesting ServiceCreate")

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("ServiceCreate", org), string(b))
	if err != nil {
		return resp, err
	}

	c.log.V(5).WithValues("response", req, "org", org).Info("received ServiceCreate")

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ServiceShow shows a service
func (c *Client) ServiceShow(org string, name string) (models.ServiceShowResponse, error) {
	var resp models.ServiceShowResponse

	data, err := c.get(api.Routes.Path("ServiceShow", org, name))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ServiceApps lists all the apps by services
func (c *Client) ServiceApps(org string) (models.ServiceAppsResponse, error) {
	resp := models.ServiceAppsResponse{}

	data, err := c.get(api.Routes.Path("ServiceApps", org))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}
