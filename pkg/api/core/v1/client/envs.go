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

	"github.com/pkg/errors"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// EnvList returns a map of all env vars for an app
func (c *Client) EnvList(namespace string, appName string) (models.EnvVariableMap, error) {
	var resp models.EnvVariableMap

	data, err := c.get(api.Routes.Path("EnvList", namespace, appName))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// EnvSet set env vars for an app
func (c *Client) EnvSet(req models.EnvVariableMap, namespace string, appName string) (models.Response, error) {
	resp := models.Response{}

	b, err := json.Marshal(req)
	if err != nil {
		return resp, nil
	}

	data, err := c.post(api.Routes.Path("EnvSet", namespace, appName), string(b))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, errors.Wrap(err, "response body is not JSON")
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// EnvShow shows an env variable
func (c *Client) EnvShow(namespace string, appName string, envName string) (models.EnvVariable, error) {
	resp := models.EnvVariable{}

	data, err := c.get(api.Routes.Path("EnvShow", namespace, appName, envName))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// EnvUnset removes an env var
func (c *Client) EnvUnset(namespace string, appName string, envName string) (models.Response, error) {
	resp := models.Response{}

	data, err := c.delete(api.Routes.Path("EnvUnset", namespace, appName, envName))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// EnvMatch returns all env vars matching the prefix
func (c *Client) EnvMatch(namespace string, appName string, prefix string) (models.EnvMatchResponse, error) {
	resp := models.EnvMatchResponse{}

	data, err := c.get(api.Routes.Path("EnvMatch", namespace, appName, prefix))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}
