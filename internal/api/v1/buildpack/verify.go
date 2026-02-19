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

package buildpack

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/registry"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Verify handles GET /api/v1/buildpacks/verify?name=<buildpack_name>
// It verifies the buildpack name exists on Docker Hub. CNB registry uses dashes in org (e.g. paketo-buildpacks)
// while Docker Hub often uses no dashes (e.g. paketobuildpacks); we try both.
func Verify(c *gin.Context) apierror.APIErrors {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		response.OKReturn(c, models.BuildpackVerifyResponse{
			Valid:   false,
			Message: "buildpack name is required",
		})
		return nil
	}
	parts := strings.SplitN(name, "/", 2)
	if len(parts) != 2 {
		response.OKReturn(c, models.BuildpackVerifyResponse{
			Valid:   false,
			Message: "buildpack name must be in form namespace/name (e.g. paketo-buildpacks/nodejs)",
		})
		return nil
	}
	ns, repoName := parts[0], parts[1]
	normalizedNs := strings.ReplaceAll(ns, "-", "")
	candidates := []string{
		"docker.io/" + normalizedNs + "/" + repoName + ":latest",
		"docker.io/" + ns + "/" + repoName + ":latest",
	}
	ctx := c.Request.Context()
	for _, imageRef := range candidates {
		exists, err := registry.ImageExistsInRegistry(ctx, imageRef)
		if err != nil {
			continue
		}
		if exists {
			response.OKReturn(c, models.BuildpackVerifyResponse{
				Valid:          true,
				NormalizedName: normalizedNs + "/" + repoName,
			})
			return nil
		}
	}
	response.OKReturn(c, models.BuildpackVerifyResponse{
		Valid:   false,
		Message: "buildpack image not found on Docker Hub (tried " + normalizedNs + "/" + repoName + " and " + ns + "/" + repoName + ")",
	})
	return nil
}
