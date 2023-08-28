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
	"fmt"
	"net/url"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

func (c *Client) ServiceCatalog() (models.CatalogServices, error) {
	response := models.CatalogServices{}
	endpoint := api.Routes.Path("ServiceCatalog")

	return Get(c, endpoint, response)
}

func (c *Client) ServiceCatalogShow(serviceName string) (*models.CatalogService, error) {
	response := &models.CatalogService{}
	endpoint := api.Routes.Path("ServiceCatalogShow", serviceName)

	return Get(c, endpoint, response)
}

// ServiceCatalogMatch returns all matching namespaces for the prefix
func (c *Client) ServiceCatalogMatch(prefix string) (models.CatalogMatchResponse, error) {
	response := models.CatalogMatchResponse{}
	endpoint := api.Routes.Path("ServiceCatalogMatch", prefix)

	return Get(c, endpoint, response)
}

func (c *Client) AllServices() (models.ServiceList, error) {
	response := models.ServiceList{}
	endpoint := api.Routes.Path("AllServices")

	return Get(c, endpoint, response)
}

func (c *Client) ServiceCreate(request models.ServiceCreateRequest, namespace string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("ServiceCreate", namespace)

	return Post(c, endpoint, request, response)
}

// ServiceUpdate updates a service by invoking the associated API endpoint
func (c *Client) ServiceUpdate(request models.ServiceUpdateRequest, namespace, name string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("ServiceUpdate", namespace, name)

	return Patch(c, endpoint, request, response)
}

func (c *Client) ServiceShow(namespace, name string) (*models.Service, error) {
	response := &models.Service{}
	endpoint := api.Routes.Path("ServiceShow", namespace, name)

	return Get(c, endpoint, response)
}

// ServiceMatch returns all matching services for the prefix
func (c *Client) ServiceMatch(namespace, prefix string) (models.ServiceMatchResponse, error) {
	response := models.ServiceMatchResponse{}
	endpoint := api.Routes.Path("ServiceMatch", namespace, prefix)

	return Get(c, endpoint, response)
}

func (c *Client) ServiceDelete(request models.ServiceDeleteRequest, namespace string, names []string) (models.ServiceDeleteResponse, error) {
	response := models.ServiceDeleteResponse{}

	queryParams := url.Values{}
	for _, serviceName := range names {
		queryParams.Add("services[]", serviceName)
	}

	endpoint := fmt.Sprintf(
		"%s?%s",
		api.Routes.Path("ServiceBatchDelete", namespace),
		queryParams.Encode(),
	)

	return Delete(c, endpoint, request, response)
}

func (c *Client) ServiceBind(request models.ServiceBindRequest, namespace, name string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("ServiceBind", namespace, name)

	return Post(c, endpoint, request, response)
}

func (c *Client) ServiceUnbind(request models.ServiceUnbindRequest, namespace, name string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("ServiceUnbind", namespace, name)

	return Post(c, endpoint, request, response)
}

func (c *Client) ServiceList(namespace string) (models.ServiceList, error) {
	response := models.ServiceList{}
	endpoint := api.Routes.Path("ServiceList", namespace)

	return Get(c, endpoint, response)
}

// ServiceApps lists a map from services to bound apps, for the namespace
func (c *Client) ServiceApps(namespace string) (models.ServiceAppsResponse, error) {
	response := models.ServiceAppsResponse{}
	endpoint := api.Routes.Path("ServiceApps", namespace)

	return Get(c, endpoint, response)
}

// ServicePortForward will forward the local traffic to a remote app
func (c *Client) ServicePortForward(namespace string, serviceName string, opts *PortForwardOpts) error {
	endpoint := fmt.Sprintf("%s%s/%s", c.Settings.API, api.WsRoot, api.WsRoutes.Path("ServicePortForward", namespace, serviceName))

	if fw, err := NewServicePortForwarder(c, endpoint, opts.Address, opts.Ports, opts.StopChannel); err != nil {
		return err
	} else {
		return fw.ForwardPorts()
	}
}
