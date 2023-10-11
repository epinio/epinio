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
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Info returns information about Epinio and its components
func (c *Client) Info() (models.InfoResponse, error) {
	response := models.InfoResponse{}
	endpoint := "info"

	return Get(c, endpoint, response)
}

// Me returns the current user
func (c *Client) Me() (models.MeResponse, error) {
	response := models.MeResponse{}
	endpoint := "me"

	return Get(c, endpoint, response)
}
