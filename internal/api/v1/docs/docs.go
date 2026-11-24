// Copyright © 2021 - 2026 KRUMWARE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Epinio API
//
// The Epinio server API, used by the CLI, the dashboard and the MCP server.
//
// Requests authenticate with Basic Auth. Lifecycle
// requests (stage, deploy, restart, update, source patch, and changes that
// redeploy bound apps) answer 202 with an operation; poll AppState until
// the operation is done.
//
//     Schemes: http, https
//     Host:
//     BasePath: /api/v1
//     Version: v1
//
//     SecurityDefinitions:
//     basicAuth:
//          type: basic
//
//     Security:
//     - basicAuth: []
//
// swagger:meta
package docs

import (
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// The error body every failed request returns.
// swagger:response ErrorResponse
type ErrorResponse struct {
	// in: body
	Body apierrors.ErrorResponse
}
