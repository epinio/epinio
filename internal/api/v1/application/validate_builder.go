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

package application

import (
	"strings"

	"github.com/gin-gonic/gin"
	parser "github.com/novln/docker-parser"

	"github.com/epinio/epinio/internal/api/v1/response"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// ValidateBuilderImageResult holds the result of validating a builder image name.
// It can be extended to include registry-index lookups (e.g. buildpacks.io)
// for checking against a list of supported builders.
type ValidateBuilderImageResult struct {
	Valid      bool   `json:"valid"`
	Message    string `json:"message,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// ValidateBuilderImage checks that the builder image is a valid container image
// reference. Invalid references (e.g. wildcards like "paketobuildpacks/builder:*")
// cause staging to fail later with InvalidImageName; this allows callers to fail
// fast with a clear error and suggestion.
// See: https://github.com/epinio/epinio/issues/2711
// Optional: validation against buildpacks.io registry-index can be added to
// ensure the image is a known supported builder.
func ValidateBuilderImage(builderImage string) ValidateBuilderImageResult {
	if builderImage == "" {
		return ValidateBuilderImageResult{
			Valid:      false,
			Message:    "builder image name is empty",
			Suggestion: "Use a specific image and tag, e.g. paketobuildpacks/builder:full or paketobuildpacks/builder-jammy-full:latest",
		}
	}

	ref, err := parser.Parse(builderImage)
	if err != nil {
		return ValidateBuilderImageResult{
			Valid:      false,
			Message:    "invalid image reference: " + err.Error(),
			Suggestion: "Use a specific image and tag (no wildcards). Example: paketobuildpacks/builder:full",
		}
	}

	// Reject wildcards in tag (e.g. "paketobuildpacks/builder:*") which
	// docker-parser might accept but Kubernetes does not.
	tag := ref.Tag()
	if strings.Contains(tag, "*") {
		return ValidateBuilderImageResult{
			Valid:      false,
			Message:    "builder image tag must not contain wildcards (e.g. *)",
			Suggestion: "Use a specific tag, e.g. paketobuildpacks/builder:full",
		}
	}

	// Tag may be empty (implies "latest"); that's valid.
	return ValidateBuilderImageResult{Valid: true}
}

// ValidateBuilderImageHandler handles GET /api/v1/validate-builder-image?image=<builder-image>
// It returns whether the builder image name is valid before attempting to stage.
// See: https://github.com/epinio/epinio/issues/2711
func ValidateBuilderImageHandler(c *gin.Context) apierror.APIErrors {
	image := c.Query("image")
	result := ValidateBuilderImage(image)
	response.OKReturn(c, result)
	return nil
}
