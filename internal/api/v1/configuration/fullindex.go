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

package configuration

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"

	"github.com/gin-gonic/gin"
)

// FullIndex handles the API endpoint GET /configurations
// It lists all the known applications in all namespaces, with and without workload.
func FullIndex(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	user := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	allConfigurations, err := configurations.List(ctx, cluster, "")
	if err != nil {
		return apierror.InternalError(err)
	}
	filteredConfigurations := auth.FilterResources(user, allConfigurations)

	if page, pageSize, ok := response.GetPaginationParams(c, 1, 25); ok {
		total := len(filteredConfigurations)
		start := (page - 1) * pageSize
		if start >= total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		pageConfigurations := filteredConfigurations[start:end]

		// Scope the binding lookup to just the namespaces on this page.
		nsSet := map[string]struct{}{}
		for _, cfg := range pageConfigurations {
			nsSet[cfg.Namespace()] = struct{}{}
		}
		pageNamespaces := make([]string, 0, len(nsSet))
		for ns := range nsSet {
			pageNamespaces = append(pageNamespaces, ns)
		}

		appsOf, err := application.BoundAppsNamesForNamespaces(ctx, cluster, pageNamespaces)
		if err != nil {
			return apierror.InternalError(err)
		}

		// Use all filtered configs for sibling map; Details() called only for page configs.
		responseData, err := makeResponseFrom(ctx, appsOf, filteredConfigurations, pageConfigurations)
		if err != nil {
			return apierror.InternalError(err)
		}

		response.OKReturn(c, response.BuildPaginatedResponse(responseData, page, pageSize, total))
		return nil
	}

	appsOf, err := application.BoundAppsNames(ctx, cluster, "")
	if err != nil {
		return apierror.InternalError(err)
	}

	responseData, err := makeResponse(ctx, appsOf, filteredConfigurations)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKReturn(c, responseData)
	return nil
}
