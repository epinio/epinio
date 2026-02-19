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
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	parser "github.com/novln/docker-parser"

	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/registry"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

var imageExistsInRegistryFn = registry.ImageExistsInRegistry

// ValidateBuilderImageResult holds the result of validating a builder image name.
// It can be extended to include registry-index lookups (e.g. buildpacks.io)
// for checking against a list of supported builders.
type ValidateBuilderImageResult struct {
	Valid      bool   `json:"valid"`
	Message    string `json:"message,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// ValidateBuilderImage checks that the builder image is a valid container image
// reference and, when ctx is non-nil, that the image exists in the container
// registry (Docker Hub, GHCR, etc.). The buildpacks registry-index
// (https://github.com/buildpacks/registry-index) indexes buildpacks, not builder
// images; existence is checked via the Registry API v2.
// See: https://github.com/epinio/epinio/issues/2711
func ValidateBuilderImage(builderImage string) ValidateBuilderImageResult {
	return ValidateBuilderImageWithContext(context.Background(), builderImage, false)
}

// ValidateBuilderImageWithContext performs format validation and, when checkRegistry
// is true, checks that the image exists in the container registry using ctx.
func ValidateBuilderImageWithContext(ctx context.Context, builderImage string, checkRegistry bool) ValidateBuilderImageResult {
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

	// Optionally check that the image exists in the container registry.
	if checkRegistry {
		exists, regErr := imageExistsInRegistryFn(ctx, builderImage)
		if regErr != nil {
			return ValidateBuilderImageResult{
				Valid:      false,
				Message:    "unable to verify image in registry",
				Suggestion: "Try again in a few seconds. If the issue persists, check registry/network connectivity.",
			}
		}
		if !exists {
			return ValidateBuilderImageResult{
				Valid:      false,
				Message:    "image not found in registry",
				Suggestion: "Check the image name and tag, or use a known builder e.g. paketobuildpacks/builder:full",
			}
		}
	}

	return ValidateBuilderImageResult{Valid: true}
}

// ValidateBuilderImageHandler handles GET /api/v1/validate-builder-image?image=<builder-image>
// It returns whether the builder image is valid before attempting to stage.
func ValidateBuilderImageHandler(c *gin.Context) apierror.APIErrors {
	image := strings.TrimSpace(c.Query("image"))
	result := ValidateBuilderImageWithContext(c.Request.Context(), image, true)
	response.OKReturn(c, result)
	return nil
}
