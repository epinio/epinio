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

// BuilderImageList returns a list of all known builder images.
func (c *Client) BuilderImageList() (models.BuilderImageList, error) {
	response := models.BuilderImageList{}
	endpoint := api.Routes.Path("BuilderImageList")

	return Get(c, endpoint, response)
}

// BuilderImageShow returns the named builder image.
func (c *Client) BuilderImageShow(name string) (models.BuilderImage, error) {
	response := models.BuilderImage{}
	endpoint := api.Routes.Path("BuilderImageShow", name)

	return Get(c, endpoint, response)
}

// BuilderImageMatch returns all builder images whose name matches the prefix.
func (c *Client) BuilderImageMatch(prefix string) (models.BuilderImageMatchResponse, error) {
	response := models.BuilderImageMatchResponse{}
	endpoint := api.Routes.Path("BuilderImageMatch", prefix)

	return Get(c, endpoint, response)
}

// BuilderImageCreate creates a builder image. The name travels in the request
// body.
func (c *Client) BuilderImageCreate(request models.BuilderImageCreateRequest) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("BuilderImageCreate")

	return Post(c, endpoint, request, response)
}

// BuilderImageUpdate updates the named builder image. The name travels in the
// URL; omitted request fields leave the corresponding values unchanged.
func (c *Client) BuilderImageUpdate(name string, request models.BuilderImageUpdateRequest) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("BuilderImageUpdate", name)

	return Patch(c, endpoint, request, response)
}

// BuilderImageDelete deletes the named builder image.
func (c *Client) BuilderImageDelete(name string) (models.Response, error) {
	response := models.Response{}
	endpoint := api.Routes.Path("BuilderImageDelete", name)

	return Delete(c, endpoint, nil, response)
}
