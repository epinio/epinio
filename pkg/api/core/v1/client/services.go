// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"encoding/json"
	"fmt"
	"net/url"

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

// ServiceCatalogMatch returns all matching namespaces for the prefix
func (c *Client) ServiceCatalogMatch(prefix string) (models.CatalogMatchResponse, error) {
	resp := models.CatalogMatchResponse{}

	data, err := c.get(api.Routes.Path("ServiceCatalogMatch", prefix))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
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

// ServiceMatch returns all matching services for the prefix
func (c *Client) ServiceMatch(namespace, prefix string) (models.ServiceMatchResponse, error) {
	resp := models.ServiceMatchResponse{}

	data, err := c.get(api.Routes.Path("ServiceMatch", namespace, prefix))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

func (c *Client) ServiceDelete(req models.ServiceDeleteRequest, namespace string, names []string, f ErrorFunc) (models.ServiceDeleteResponse, error) {

	resp := models.ServiceDeleteResponse{}

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	URL := constructServiceBatchDeleteURL(namespace, names)

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

func constructServiceBatchDeleteURL(namespace string, names []string) string {
	q := url.Values{}
	for _, c := range names {
		q.Add("services[]", c)
	}
	URLParams := q.Encode()

	URL := api.Routes.Path("ServiceBatchDelete", namespace)

	return fmt.Sprintf("%s?%s", URL, URLParams)
}
