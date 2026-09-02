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

package docs

//go:generate swagger generate spec

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// swagger:route POST /gitproxy gitproxy GitProxy
// Proxy a GET request to the posted git provider `url`, optionally authenticating
// with the named `gitconfig`. The caller must be allowed to use that git
// configuration. The provider's response body is returned unchanged.
// responses:
//   200: GitProxyResponse

// swagger:parameters GitProxy
type GitProxyParam struct {
	// in: body
	Body models.GitProxyRequest
}

// swagger:response GitProxyResponse
type GitProxyResponse struct {
	// in: body
	Body []byte
}
