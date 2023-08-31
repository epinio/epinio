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

// GitconfigDelete deletes a gitconfig
func (c *Client) GitconfigDelete(gitconfigs []string) (models.Response, error) {
	response := models.Response{}

	queryParams := url.Values{}
	for _, gitconfig := range gitconfigs {
		queryParams.Add("gitconfigs[]", gitconfig)
	}

	endpoint := fmt.Sprintf(
		"%s?%s",
		api.Routes.Path("GitconfigBatchDelete"),
		queryParams.Encode(),
	)

	return Delete(c, endpoint, nil, response)
}

// GitconfigShow shows a gitconfig
func (c *Client) GitconfigShow(gitconfig string) (models.Gitconfig, error) {
	response := models.Gitconfig{}
	endpoint := api.Routes.Path("GitconfigShow", gitconfig)

	return Get(c, endpoint, response)
}

// GitconfigsMatch returns all matching gitconfigs for the prefix
func (c *Client) GitconfigsMatch(prefix string) (models.GitconfigsMatchResponse, error) {
	response := models.GitconfigsMatchResponse{}
	endpoint := api.Routes.Path("GitconfigsMatch", prefix)

	return Get(c, endpoint, response)
}

// Gitconfigs returns a list of gitconfigs
func (c *Client) Gitconfigs() (models.GitconfigList, error) {
	response := models.GitconfigList{}
	endpoint := api.Routes.Path("Gitconfigs")

	return Get(c, endpoint, response)
}

// GitconfigCreate creates a gitconfig
func (c *Client) GitconfigCreate(request models.GitconfigCreateRequest) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("GitconfigCreate")
	return Post(c, endpoint, request, response)
}
