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

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// NamespaceCreate creates a namespace
func (c *Client) NamespaceCreate(req models.NamespaceCreateRequest) (models.Response, error) {
	var resp models.Response

	b, err := json.Marshal(req)
	if err != nil {
		return resp, err
	}

	data, err := c.post(api.Routes.Path("Namespaces"), string(b))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// NamespaceDelete deletes a namespace
func (c *Client) NamespaceDelete(namespaces []string) (models.Response, error) {
	resp := models.Response{}

	URL := constructNamespaceBatchDeleteURL(namespaces)

	data, err := c.delete(URL)
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// NamespaceShow shows a namespace
func (c *Client) NamespaceShow(namespace string) (models.Namespace, error) {
	resp := models.Namespace{}

	data, err := c.get(api.Routes.Path("NamespaceShow", namespace))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// NamespacesMatch returns all matching namespaces for the prefix
func (c *Client) NamespacesMatch(prefix string) (models.NamespacesMatchResponse, error) {
	resp := models.NamespacesMatchResponse{}

	data, err := c.get(api.Routes.Path("NamespacesMatch", prefix))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

// Namespaces returns a list of namespaces
func (c *Client) Namespaces() (models.NamespaceList, error) {
	resp := models.NamespaceList{}

	data, err := c.get(api.Routes.Path("Namespaces"))
	if err != nil {
		return resp, err
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return resp, err
	}

	c.log.V(1).Info("response decoded", "response", resp)

	return resp, nil
}

func constructNamespaceBatchDeleteURL(namespaces []string) string {
	q := url.Values{}
	for _, c := range namespaces {
		q.Add("namespaces[]", c)
	}
	URLParams := q.Encode()

	URL := api.Routes.Path("NamespaceBatchDelete")

	return fmt.Sprintf("%s?%s", URL, URLParams)
}
