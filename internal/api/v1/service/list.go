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

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

func List(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	serviceList, err := kubeServiceClient.ListInNamespace(ctx, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	appsOf, err := application.ServicesBoundAppsNames(ctx, cluster, "")
	if err != nil {
		return apierror.InternalError(err)
	}

	servicesWithApps := extendWithBoundApps(serviceList, appsOf)

	if search := response.GetSearchParam(c); search != "" {
		lower := strings.ToLower(search)
		var filtered models.ServiceList
		for _, svc := range servicesWithApps {
			if strings.Contains(strings.ToLower(svc.Meta.Name), lower) {
				filtered = append(filtered, svc)
			}
		}
		servicesWithApps = filtered
	}

	if page, pageSize, ok := response.GetPaginationParams(c, 1, 25); ok {
		paged := response.PaginateSlice(servicesWithApps, page, pageSize)
		response.OKReturn(c, paged)
		return nil
	}

	// Backwards-compatible: return full list when no page params are set.
	response.OKReturn(c, servicesWithApps)
	return nil
}
