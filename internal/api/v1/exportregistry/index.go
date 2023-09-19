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

package exportregistry

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/registry"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Index handles the API endpoint /exportregistries (GET)
// It returns a list of all export registries set up by the operator
func Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	// user := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	registries, err := registry.ExportRegistries(
		requestctx.Logger(ctx),
		cluster.Kubectl.CoreV1().Secrets(helmchart.Namespace()))
	if err != nil {
		return apierror.InternalError(err)
	}

	// Filter accessible registries by user ?
	// registries = auth.FilterResources(user, registries)

	result := models.ExportregistriesListResponse{}
	for _, registry := range registries {
		result = append(result, models.ExportregistryResponse{
			Name: registry.Name,
			URL:  registry.URL,
		})
	}

	response.OKReturn(c, result)
	return nil
}
