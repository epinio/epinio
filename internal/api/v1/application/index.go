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
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Index handles the API endpoint GET /namespaces/:namespace/applications
// It lists applications in the namespace that have been deployed at least once.
// Never-deployed (build-only) apps are returned by UndeployedIndex instead.
func Index(c *gin.Context) apierror.APIErrors {
	return indexWithFilter(c, application.ListFilterDeployed)
}

// UndeployedIndex handles GET /namespaces/:namespace/applications/undeployed
// It lists never-deployed apps (buildstatus == "build" and empty image URL)
// with optional pagination and search, for the UI Builds / undeployed-apps view.
func UndeployedIndex(c *gin.Context) apierror.APIErrors {
	return indexWithFilter(c, application.ListFilterUndeployed)
}

func indexWithFilter(c *gin.Context, filter application.ListFilter) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	search := response.GetSearchParam(c)
	page, pageSize, paginated := response.GetPaginationParams(c, 1, 25)
	if paginated || search != "" {
		apps, total, err := application.ListPaginated(
			ctx, cluster, namespace, page, pageSize, search, filter,
		)
		if err != nil {
			return apierror.InternalError(err)
		}
		response.OKReturn(c, response.BuildPaginatedResponse(apps, page, pageSize, total))
		return nil
	}

	apps, err := application.List(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, application.FilterApps(apps, filter))
	return nil
}
