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
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// EnvList returns a map of all env vars for an app
func (c *Client) EnvList(namespace string, appName string) (models.EnvVariableMap, error) {
	response := models.EnvVariableMap{}
	endpoint := api.Routes.Path("EnvList", namespace, appName)

	return Get(c, endpoint, response)
}

// EnvSet set env vars for an app
func (c *Client) EnvSet(request models.EnvVariableMap, namespace string, appName string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("EnvSet", namespace, appName)

	return Post(c, endpoint, request, response)
}

// EnvShow shows an env variable
func (c *Client) EnvShow(namespace string, appName string, envName string) (models.EnvVariable, error) {
	response := models.EnvVariable{}
	endpoint := api.Routes.Path("EnvShow", namespace, appName, envName)

	return Get(c, endpoint, response)
}

// EnvUnset removes an env var
func (c *Client) EnvUnset(namespace string, appName string, envName string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("EnvUnset", namespace, appName, envName)

	return Delete(c, endpoint, nil, response)
}

// EnvMatch returns all env vars matching the prefix
func (c *Client) EnvMatch(namespace string, appName string, prefix string) (models.EnvMatchResponse, error) {
	response := models.EnvMatchResponse{}
	endpoint := api.Routes.Path("EnvMatch", namespace, appName, prefix)

	return Get(c, endpoint, response)
}
