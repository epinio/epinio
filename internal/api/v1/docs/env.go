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

// Env -- Application Environment

// swagger:route GET /namespaces/{Namespace}/applications/{App}/environment app-env EnvList
// Return the environment variable assignments for the `App` in the namespace`.
// responses:
//   200: EnvListResponse

// swagger:parameters EnvList
type EnvListParams struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// swagger:response EnvListResponse
type EnvListResponse struct {
	// in: body
	Body models.EnvVariableMap
}

// swagger:route GET /namespaces/{Namespace}/applications/{App}/environmentmatch/{Pattern} app-env EnvMatch
// Return the environment variable names of the `App` in the `Namespace` with prefix `Pattern`.
// responses:
//   200: EnvMatchResponse

// swagger:parameters EnvMatch
type EnvMatchParams struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: path
	Pattern string
}

// swagger:response EnvMatchResponse
type EnvMatchResponse struct {
	// in: body
	Body models.EnvMatchResponse
}

// swagger:route GET /namespaces/{Namespace}/applications/{App}/environmentmatch app-env EnvMatch0
// Return the environment variable names of the `App` in the `Namespace`. (No prefix == Empty prefix == All match)
// responses:
//   200: EnvMatchResponse

// swagger:parameters EnvMatch0
type EnvMatch0Params struct {
	// in: path
	Namespace string
	// in: path
	App string
}

// See EnvMatch above

// swagger:route POST /namespaces/{Namespace}/applications/{App}/environment app-env EnvSet
// Create/modify the posted environment variable assignments for the `App` in the `Namespace`.
// responses:
//   200: EnvSetResponse

// swagger:parameters EnvSet
type EnvSetParams struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: body
	Body models.EnvVariableMap
}

// swagger:response EnvSetResponse
type EnvSetResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /namespaces/{Namespace}/applications/{App}/environment/{Env} app-env EnvShow
// Return the named `Env` variable assignment for the `App` in the `Namespace`.
// responses:
//   200: EnvShowResponse

// swagger:parameters EnvShow
type EnvShowParams struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: path
	Env string
}

// swagger:response EnvShowResponse
type EnvShowResponse struct {
	// in: body
	Body models.EnvVariable
}

// swagger:route DELETE /namespaces/{Namespace}/applications/{App}/environment/{Env} app-env EnvUnset
// Remove the named `Env` variable from the `App` in the `Namespace`.
// responses:
//   200: EnvUnsetResponse

// swagger:parameters EnvUnset
type EnvUnsetParams struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: path
	Env string
}

// swagger:response EnvUnsetResponse
type EnvUnsetResponse struct {
	// in: body
	Body models.Response
}
