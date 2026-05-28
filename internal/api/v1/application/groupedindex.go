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
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// GroupedIndex handles GET /applications/grouped
// Returns a paginated app list per namespace in a single call, with each namespace
// queried concurrently. Replaces N per-namespace calls on the applications list page.
func GroupedIndex(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	user := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	page, pageSize, _ := response.GetPaginationParams(c, 1, 10)
	search := response.GetSearchParam(c)

	grouped, err := application.ListPaginatedByNamespace(ctx, cluster, page, pageSize, search)
	if err != nil {
		return apierror.InternalError(err)
	}

	result := make(map[string]response.PaginatedResponse[models.App], len(grouped))
	for ns, nsResult := range grouped {
		filtered := auth.FilterResources(user, nsResult.Items)
		result[ns] = response.BuildPaginatedResponse(filtered, page, pageSize, nsResult.TotalItems)
	}

	response.OKReturn(c, result)
	return nil
}
