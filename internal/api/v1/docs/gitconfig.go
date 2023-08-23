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

// swagger:route GET /gitconfigs gitconfig Gitconfigs
// Return list of all controlled gitconfigs.
// responses:
//   200: GitconfigsResponse

// swagger:response GitconfigsResponse
type GitconfigsResponse struct {
	// in: body
	Body models.GitconfigList
}

// swagger:route POST /gitconfigs gitconfig GitconfigCreate
// Create the posted new gitconfig.
// responses:
//   200: GitconfigCreateResponse

// swagger:parameters GitconfigCreate
type GitconfigCreateParam struct {
	// in: body
	Body models.GitconfigCreateRequest
}

// swagger:response GitconfigCreateResponse
type GitconfigCreateResponse struct {
	// in: body
	Body models.Response
}

// swagger:route DELETE /gitconfigs GitconfigBatchDelete
// Delete the named gitconfigs.
// responses:
//   200: GitconfigDeleteResponse

// swagger:route DELETE /gitconfigs/{Gitconfig} gitconfig GitconfigDelete
// Delete the named `Gitconfig`.
// responses:
//   200: GitconfigDeleteResponse

// swagger:parameters GitconfigDelete
type GitconfigDeleteParam struct {
	// in: path
	Gitconfig string
}

// swagger:parameters GitconfigBatchDelete
type GitconfigBatchDeleteParam struct {
	// in: url
	Gitconfigs []string
}

// swagger:response GitconfigDeleteResponse
type GitconfigDeleteResponse struct {
	// in: body
	Body models.Response
}

// swagger:route GET /gitconfigs/{Gitconfig} gitconfig Gitconfigshow
// Return details of the named `Gitconfig`.
// responses:
//   200: GitconfigshowResponse

// swagger:parameters Gitconfigshow
type GitconfigshowParam struct {
	// in: path
	Gitconfig string
}

// swagger:response GitconfigshowResponse
type GitconfigshowResponse struct {
	// in: body
	Body models.Gitconfig
}

// swagger:route GET /gitconfigmatches/{Pattern} gitconfig GitconfigMatch
// Return list of names for all controlled gitconfigs whose name matches the prefix `Pattern`.
// responses:
//   200: GitconfigMatchResponse

// swagger:parameters GitconfigMatch
type GitconfigMatchParam struct {
	// in: path
	Pattern string
}

// swagger:response GitconfigMatchResponse
type GitconfigMatchResponse struct {
	// in: body
	Body models.GitconfigsMatchResponse
}

// swagger:route GET /gitconfigmatches gitconfig GitconfigMatch0
// Return list of names for all controlled gitconfigs (No prefix = empty prefix = match everything)
// responses:
//   200: GitconfigMatchResponse

// swagger:parameters GitconfigMatch0
type GitconfigMatch0Param struct{}

// response: See GitconfigMatch.
