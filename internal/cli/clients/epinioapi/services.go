package epinioapi

import (
	"encoding/json"

	"github.com/pkg/errors"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/services"
)

// ServicePlans returns a list of service plans for a given serviceclass name
func (c *Client) ServicePlans(name string) (services.ServicePlanList, error) {
	resp := services.ServicePlanList{}

	data, err := c.get(api.Routes.Path("ServicePlans", name))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}

// ServiceClasses reutrns a list of service classes
func (c *Client) ServiceClasses() (services.ServiceClassList, error) {
	resp := services.ServiceClassList{}

	data, err := c.get(api.Routes.Path("ServiceClasses"))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	return resp, nil
}

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

	return resp, nil
}

// ServiceDelete deletes a service
func (c *Client) ServiceDelete(req models.DeleteRequest, org string, name string, f errorFunc) (models.DeleteResponse, error) {
	resp := models.DeleteResponse{}

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

	return resp, nil
}

// ServiceCreate creates a service from the catalog
func (c *Client) ServiceCreate(req models.CatalogCreateRequest, org string) (models.Response, error) {
	resp := models.Response{}

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("ServiceCreate", org), string(b))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	return resp, nil
}

// ServiceCreateCustom creates a custom service
func (c *Client) ServiceCreateCustom(req models.CustomCreateRequest, org string) (models.Response, error) {
	resp := models.Response{}

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("ServiceCreateCustom", org), string(b))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

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

	return resp, nil
}
