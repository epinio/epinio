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

// swagger:route GET /builderimages builderimages BuilderImageList
// Return list of builderimages.
// responses:
//   200: BuilderImagesResponse

// swagger:parameters BuilderImageList
type BuilderImageListParam struct{}

// swagger:response BuilderImagesResponse
type BuilderImagesResponse struct {
	// in: body
	Body models.BuilderImageList
}

// swagger:route POST /builderimages builderimages BuilderImageCreate
// Create a new builderimage.
// responses:
//   201: BuilderImageCreateResponse

// swagger:parameters BuilderImageCreate
type BuilderImageCreateParam struct {
	// in: body
	Body models.BuilderImageCreateRequest
}

// swagger:response BuilderImageCreateResponse
type BuilderImageCreateResponse struct{}

// swagger:route GET /builderimages/{BuilderImage} builderimages BuilderImageShow
// Return details of the named `BuilderImage`.
// responses:
//   200: BuilderImageShowResponse

// swagger:parameters BuilderImageShow
type BuilderImageShowParam struct {
	// in: path
	BuilderImage string
}

// swagger:response BuilderImageShowResponse
type BuilderImageShowResponse struct {
	// in: body
	Body models.BuilderImage
}

// swagger:route PATCH /builderimages/{BuilderImage} builderimages BuilderImageUpdate
// Update fields on the named `BuilderImage`.
// responses:
//   200: BuilderImageUpdateResponse

// swagger:parameters BuilderImageUpdate
type BuilderImageUpdateParam struct {
	// in: path
	BuilderImage string
	// in: body
	Body models.BuilderImageUpdateRequest
}

// swagger:response BuilderImageUpdateResponse
type BuilderImageUpdateResponse struct{}

// swagger:route DELETE /builderimages/{BuilderImage} builderimages BuilderImageDelete
// Delete the named `BuilderImage`.
// responses:
//   200: BuilderImageDeleteResponse

// swagger:parameters BuilderImageDelete
type BuilderImageDeleteParam struct {
	// in: path
	BuilderImage string
}

// swagger:response BuilderImageDeleteResponse
type BuilderImageDeleteResponse struct{}

// swagger:route GET /builderimagesmatch/{Pattern} builderimages BuilderImageMatch
// Return the builderimage names with prefix `Pattern`.
// responses:
//   200: BuilderImageMatchResponse

// swagger:parameters BuilderImageMatch
type BuilderImageMatchParams struct {
	// in: path
	Pattern string
}

// swagger:response BuilderImageMatchResponse
type BuilderImageMatchResponse struct {
	// in: body
	Body models.BuilderImageMatchResponse
}

// swagger:route GET /builderimagesmatch builderimages BuilderImageMatch0
// Return all builderimage names. (Empty prefix == all match)
// responses:
//   200: BuilderImageMatchResponse

// swagger:parameters BuilderImageMatch0
type BuilderImageMatch0Params struct{}

// See BuilderImageMatch above
