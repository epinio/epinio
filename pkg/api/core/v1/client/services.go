package client

import (
	"encoding/json"

	"github.com/pkg/errors"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Services returns a list of services for the specified namespace
func (c *Client) Services(namespace string) (models.ServiceResponseList, error) {
	resp := models.ServiceResponseList{}

	data, err := c.get(api.Routes.Path("Services", namespace))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// AllServices returns a list of all services, across all namespaces
func (c *Client) AllServices() (models.ServiceResponseList, error) {
	resp := models.ServiceResponseList{}

	data, err := c.get(api.Routes.Path("AllServices"))
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
func (c *Client) ServiceBindingCreate(req models.BindRequest, namespace string, appName string) (models.BindResponse, error) {
	resp := models.BindResponse{}

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("ServiceBindingCreate", namespace, appName), string(b))
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
func (c *Client) ServiceBindingDelete(namespace string, appName string, serviceName string) (models.Response, error) {
	resp := models.Response{}

	data, err := c.delete(api.Routes.Path("ServiceBindingDelete", namespace, appName, serviceName))
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

// ServiceCreate creates a service by invoking the associated API endpoint
func (c *Client) ServiceCreate(req models.ServiceCreateRequest, namespace string) (models.Response, error) {
	resp := models.Response{}

	c.log.V(5).WithValues("request", req, "namespace", namespace).Info("requesting ServiceCreate")

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("ServiceCreate", namespace), string(b))
	if err != nil {
		return resp, err
	}

	c.log.V(5).WithValues("response", req, "namespace", namespace).Info("received ServiceCreate")

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ServiceUpdate updates a service by invoking the associated API endpoint
func (c *Client) ServiceUpdate(req models.ServiceUpdateRequest, namespace, name string) (models.Response, error) {
	resp := models.Response{}

	c.log.V(5).WithValues("request", req, "namespace", namespace, "service", name).Info("requesting ServiceUpdate")

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.patch(api.Routes.Path("ServiceUpdate", namespace, name), string(b))
	if err != nil {
		return resp, err
	}

	c.log.V(5).WithValues("response", req, "namespace", namespace, "service", name).Info("received ServiceUpdate")

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ServiceShow shows a service
func (c *Client) ServiceShow(namespace string, name string) (models.ServiceResponse, error) {
	var resp models.ServiceResponse

	data, err := c.get(api.Routes.Path("ServiceShow", namespace, name))
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
