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
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/registry"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

var repositoryExistsInRegistryFn = registry.RepositoryExistsInRegistry

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
	candidates := []string{normalizedNs}
	if ns != normalizedNs {
		candidates = append(candidates, ns)
	}

	ctx := c.Request.Context()
	var lastErr error
	for _, candidateNs := range candidates {
		repository := candidateNs + "/" + repoName
		exists, err := repositoryExistsInRegistryFn(ctx, "docker.io", repository)
		if err != nil {
			lastErr = err
			continue
		}
		if exists {
			response.OKReturn(c, models.BuildpackVerifyResponse{
				Valid:          true,
				NormalizedName: repository,
			})
			return nil
		}
	}

	if lastErr != nil {
		helpers.Logger.Errorw("buildpack verification failed", "name", name, "error", lastErr)
		return apierror.NewAPIError("unable to verify buildpack on Docker Hub", http.StatusServiceUnavailable).
			WithDetails(lastErr.Error())
	}

	response.OKReturn(c, models.BuildpackVerifyResponse{
		Valid:   false,
		Message: "buildpack image not found on Docker Hub (tried " + normalizedNs + "/" + repoName + " and " + ns + "/" + repoName + ")",
	})
	return nil
}
