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

package appchart

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"

	"github.com/gin-gonic/gin"
)

// Index handles the API endpoint GET /appcharts
// It lists all the known appcharts in all namespaces
func Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	log.Infow("list appcharts")
	defer log.Infow("return")

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	client, clientError := cluster.ClientAppChart()
	if clientError != nil {
		return apierror.InternalError(clientError)
	}

	log.Infow("fetch appcharts")
	allApps, listError := appchart.List(ctx, client)
	if listError != nil {
		return apierror.InternalError(listError)
	}

	inUse, inUseError := application.ChartsInUse(ctx, cluster)
	if inUseError != nil {
		return apierror.InternalError(inUseError)
	}
	for i := range allApps {
		allApps[i].BoundApps = inUse[allApps[i].Meta.Name]
	}

	// Apply optional pagination when page parameters are provided.
	if page, pageSize, ok := response.GetPaginationParams(c, 1, 25); ok {
		log.Infow(
			"paginate",
			"page",
			page,
			"pageSize",
			pageSize,
			"total",
			len(allApps),
		)
		paged := response.PaginateSlice(allApps, page, pageSize)
		response.OKReturn(c, paged)
		return nil
	}

	log.Infow("deliver appcharts", "count", len(allApps))
	// Backwards-compatible: return full list when no page params are set.
	response.OKReturn(c, allApps)
	return nil
}
