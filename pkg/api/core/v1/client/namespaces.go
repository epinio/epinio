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

// NamespaceCreate creates a namespace
func (c *Client) NamespaceCreate(request models.NamespaceCreateRequest) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("Namespaces")

	return Post(c, endpoint, request, response)
}

// NamespaceDelete deletes a namespace
func (c *Client) NamespaceDelete(namespaces []string) (models.Response, error) {
	response := models.Response{}

	queryParams := url.Values{}
	for _, namespace := range namespaces {
		queryParams.Add("namespaces[]", namespace)
	}

	endpoint := fmt.Sprintf(
		"%s?%s",
		api.Routes.Path("NamespaceBatchDelete"),
		queryParams.Encode(),
	)

	return Delete(c, endpoint, nil, response)
}

// NamespaceShow shows a namespace
func (c *Client) NamespaceShow(namespace string) (models.Namespace, error) {
	response := models.Namespace{}
	endpoint := api.Routes.Path("NamespaceShow", namespace)

	return Get(c, endpoint, response)
}

// NamespacesMatch returns all matching namespaces for the prefix
func (c *Client) NamespacesMatch(prefix string) (models.NamespacesMatchResponse, error) {
	response := models.NamespacesMatchResponse{}
	endpoint := api.Routes.Path("NamespacesMatch", prefix)

	return Get(c, endpoint, response)
}

// Namespaces returns a list of namespaces
func (c *Client) Namespaces() (models.NamespaceList, error) {
	response := models.NamespaceList{}
	endpoint := api.Routes.Path("Namespaces")

	return Get(c, endpoint, response)
}
