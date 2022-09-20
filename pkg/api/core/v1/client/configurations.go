package client

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/pkg/errors"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Configurations returns a list of configurations for the specified namespace
func (c *Client) Configurations(namespace string) (models.ConfigurationResponseList, error) {
	resp := models.ConfigurationResponseList{}

	data, err := c.get(api.Routes.Path("Configurations", namespace))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// AllConfigurations returns a list of all configurations, across all namespaces
func (c *Client) AllConfigurations() (models.ConfigurationResponseList, error) {
	resp := models.ConfigurationResponseList{}

	data, err := c.get(api.Routes.Path("AllConfigurations"))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ConfigurationBindingCreate creates a binding from an app to a configurationclass
func (c *Client) ConfigurationBindingCreate(req models.BindRequest, namespace string, appName string) (models.BindResponse, error) {
	resp := models.BindResponse{}

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("ConfigurationBindingCreate", namespace, appName), string(b))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ConfigurationBindingDelete deletes a binding from an app to a configurationclass
func (c *Client) ConfigurationBindingDelete(namespace string, appName string, configurationName string) (models.Response, error) {
	resp := models.Response{}

	data, err := c.delete(api.Routes.Path("ConfigurationBindingDelete", namespace, appName, configurationName))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ConfigurationDelete deletes a configuration
func (c *Client) ConfigurationDelete(req models.ConfigurationDeleteRequest, namespace string, names []string, f ErrorFunc) (models.ConfigurationDeleteResponse, error) {
	resp := models.ConfigurationDeleteResponse{}

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	URL := constructBatchDeleteURL(namespace, names)

	data, err := c.doWithCustomErrorHandling(URL, "DELETE", string(b), f)
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

// ConfigurationCreate creates a configuration by invoking the associated API endpoint
func (c *Client) ConfigurationCreate(req models.ConfigurationCreateRequest, namespace string) (models.Response, error) {
	resp := models.Response{}

	c.log.V(5).WithValues("request", req, "namespace", namespace).Info("requesting ConfigurationCreate")

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("ConfigurationCreate", namespace), string(b))
	if err != nil {
		return resp, err
	}

	c.log.V(5).WithValues("response", req, "namespace", namespace).Info("received ConfigurationCreate")

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ConfigurationUpdate updates a configuration by invoking the associated API endpoint
func (c *Client) ConfigurationUpdate(req models.ConfigurationUpdateRequest, namespace, name string) (models.Response, error) {
	resp := models.Response{}

	c.log.V(5).WithValues("request", req, "namespace", namespace, "configuration", name).Info("requesting ConfigurationUpdate")

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.patch(api.Routes.Path("ConfigurationUpdate", namespace, name), string(b))
	if err != nil {
		return resp, err
	}

	c.log.V(5).WithValues("response", req, "namespace", namespace, "configuration", name).Info("received ConfigurationUpdate")

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ConfigurationShow shows a configuration
func (c *Client) ConfigurationShow(namespace string, name string) (models.ConfigurationResponse, error) {
	var resp models.ConfigurationResponse

	data, err := c.get(api.Routes.Path("ConfigurationShow", namespace, name))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ConfigurationApps lists all the apps by configurations
func (c *Client) ConfigurationApps(namespace string) (models.ConfigurationAppsResponse, error) {
	resp := models.ConfigurationAppsResponse{}

	data, err := c.get(api.Routes.Path("ConfigurationApps", namespace))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// ConfigurationMatch returns all matching configurations for the prefix
func (c *Client) ConfigurationMatch(namespace, prefix string) (models.ConfigurationMatchResponse, error) {
	resp := models.ConfigurationMatchResponse{}

	data, err := c.get(api.Routes.Path("ConfigurationMatch", namespace, prefix))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

func constructBatchDeleteURL(namespace string, names []string) string {
	q := url.Values{}
	for _, c := range names {
		q.Add("configurations[]", c)
	}
	URLParams := q.Encode()

	URL := api.Routes.Path("ConfigurationBatchDelete", namespace)

	return fmt.Sprintf("%s?%s", URL, URLParams)
}
