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

package service

import (
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/services"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Match handles the API endpoint /namespace/:namespace/servicesmatches/:pattern (GET)
// It returns a list of all Epinio-controlled services matching the prefix pattern.
func Match(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	namespace := c.Param("namespace")

	helpers.Logger.Infow("match services")
	defer helpers.Logger.Infow("return")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	helpers.Logger.Infow("list services")
	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	serviceList, err := kubeServiceClient.ListInNamespace(ctx, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	helpers.Logger.Infow("get service prefix")
	prefix := c.Param("pattern")

	helpers.Logger.Infow("match prefix", "pattern", prefix)
	matches := []string{}
	for _, service := range serviceList {
		if strings.HasPrefix(service.Meta.Name, prefix) {
			matches = append(matches, service.Meta.Name)
		}
	}

	helpers.Logger.Infow("deliver matches", "found", matches)

	response.OKReturn(c, models.ServiceMatchResponse{
		Names: matches,
	})
	return nil
}
