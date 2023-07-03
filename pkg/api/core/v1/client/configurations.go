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

// Configurations returns a list of configurations for the specified namespace
func (c *Client) Configurations(namespace string) (models.ConfigurationResponseList, error) {
	v := models.ConfigurationResponseList{}
	endpoint := api.Routes.Path("Configurations", namespace)

	return Get(c, endpoint, v)
}

// AllConfigurations returns a list of all configurations, across all namespaces
func (c *Client) AllConfigurations() (models.ConfigurationResponseList, error) {
	response := models.ConfigurationResponseList{}
	endpoint := api.Routes.Path("AllConfigurations")

	return Get(c, endpoint, response)
}

// ConfigurationBindingCreate creates a binding from an app to a configurationclass
func (c *Client) ConfigurationBindingCreate(request models.BindRequest, namespace string, appName string) (models.BindResponse, error) {
	response := models.BindResponse{}
	endpoint := api.Routes.Path("ConfigurationBindingCreate", namespace, appName)

	return Post(c, endpoint, request, response)
}

// ConfigurationBindingDelete deletes a binding from an app to a configurationclass
func (c *Client) ConfigurationBindingDelete(namespace string, appName string, configurationName string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("ConfigurationBindingDelete", namespace, appName, configurationName)

	return Delete(c, endpoint, nil, response)
}

// ConfigurationDelete deletes a configuration
func (c *Client) ConfigurationDelete(req models.ConfigurationDeleteRequest, namespace string, names []string) (models.ConfigurationDeleteResponse, error) {
	response := models.ConfigurationDeleteResponse{}

	queryParams := url.Values{}
	for _, c := range names {
		queryParams.Add("configurations[]", c)
	}

	endpoint := fmt.Sprintf(
		"%s?%s",
		api.Routes.Path("ConfigurationBatchDelete", namespace),
		queryParams.Encode(),
	)

	return Delete(c, endpoint, req, response)
}

// ConfigurationCreate creates a configuration by invoking the associated API endpoint
func (c *Client) ConfigurationCreate(request models.ConfigurationCreateRequest, namespace string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("ConfigurationCreate", namespace)

	return Post(c, endpoint, request, response)
}

// ConfigurationUpdate updates a configuration by invoking the associated API endpoint
func (c *Client) ConfigurationUpdate(request models.ConfigurationUpdateRequest, namespace, name string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("ConfigurationUpdate", namespace, name)

	return Patch(c, endpoint, request, response)
}

// ConfigurationShow shows a configuration
func (c *Client) ConfigurationShow(namespace string, name string) (models.ConfigurationResponse, error) {
	v := models.ConfigurationResponse{}
	endpoint := api.Routes.Path("ConfigurationShow", namespace, name)

	return Get(c, endpoint, v)
}

// ConfigurationApps lists all the apps by configurations
func (c *Client) ConfigurationApps(namespace string) (models.ConfigurationAppsResponse, error) {
	v := models.ConfigurationAppsResponse{}
	endpoint := api.Routes.Path("ConfigurationApps", namespace)

	return Get(c, endpoint, v)
}

// ConfigurationMatch returns all matching configurations for the prefix
func (c *Client) ConfigurationMatch(namespace, prefix string) (models.ConfigurationMatchResponse, error) {
	v := models.ConfigurationMatchResponse{}
	endpoint := api.Routes.Path("ConfigurationMatch", namespace, prefix)

	return Get(c, endpoint, v)
}
