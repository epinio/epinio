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

// swagger:route GET /authtoken auth AuthToken
// Return a JWT token for the authenticated user, for use by subsequent logins.
// responses:
//   200: AuthTokenResponse

// swagger:parameters AuthToken
type AuthTokenParam struct{}

// swagger:response AuthTokenResponse
type AuthTokenResponse struct {
	// in: body
	Body models.AuthTokenResponse
}

// swagger:route GET /me auth Me
// Return the current user, their roles, and the namespaces and git configurations
// they can access. The synthetic "default" action is not reported.
// responses:
//   200: MeResponse

// swagger:parameters Me
type MeParam struct{}

// swagger:response MeResponse
type MeResponse struct {
	// in: body
	Body models.MeResponse
}
