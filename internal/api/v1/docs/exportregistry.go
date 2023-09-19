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

import "github.com/epinio/epinio/pkg/api/core/v1/models"

//go:generate swagger generate spec

// swagger:route GET /exportregistries exportregistry Exportregistries
// Return list of all export registries
// responses:
//   200: ExportregistriesResponse

// swagger:response ExportregistriesResponse
type ExportregistriesResponse struct {
	// in: body
	Body models.ExportregistriesListResponse
}

// swagger:route GET /exportregistrymatches/{Pattern} exportregistry ExportregistryMatch
// Return list of names for all controlled exportregistries whose name matches the prefix `Pattern`.
// responses:
//   200: ExportregistryMatchResponse

// swagger:parameters ExportregistryMatch
type ExportregistryMatchParam struct {
	// in: path
	Pattern string
}

// swagger:response ExportregistryMatchResponse
type ExportregistryMatchResponse struct {
	// in: body
	Body models.ExportregistriesMatchResponse
}

// swagger:route GET /exportregistrymatches exportregistry ExportregistryMatch0
// Return list of names for all controlled exportregistries (No prefix = empty prefix = match everything)
// responses:
//   200: ExportregistryMatchResponse

// swagger:parameters ExportregistryMatch0
type ExportregistryMatch0Param struct{}

// response: See ExportregistryMatch.
