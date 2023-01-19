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

// Configuration Bindings

// swagger:route POST   /namespaces/{Namespace}/applications/{App}/configurationbindings svc-binding ConfigurationBindingCreate
// Create configuration binding between `App` in `Namespace`, and the posted configurations, also in `Namespace`.
// responses:
//   200: ConfigurationBindResponse

// swagger:parameters ConfigurationBindingCreate
type ConfigurationBindingCreateParams struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: body
	Body models.BindRequest
}

// swagger:response ConfigurationBindResponse
type ConfigurationBindResponse struct {
	// in: body
	Body models.BindResponse
}

// swagger:route DELETE /namespaces/{Namespace}/applications/{App}/configurationbindings/{Configuration} svc-binding ConfigurationBindingDelete
// Remove configuration binding between `App` and `Configuration` in `Namespace`.
// responses:
//   200: ConfigurationUnbindReponse

// swagger:parameters ConfigurationBindingDelete
type ConfigurationBindingDeleteParams struct {
	// in: path
	Namespace string
	// in: path
	App string
	// in: path
	Configuration string
}

// swagger:response ConfigurationUnbindReponse
type ConfigurationUnbindReponse struct {
	// in:body
	Body models.Response
}
