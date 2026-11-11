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

package namespace

import (
	"context"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/namespaces"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	"github.com/gin-gonic/gin"
)

// Index handles the API endpoint /namespaces (GET)
// It returns a list of all Epinio-controlled namespaces
// An Epinio namespace is nothing but a kubernetes namespace which has a
// special Label (Look at the code to see which).
func Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	user := requestctx.User(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	namespaceList, err := namespaces.List(ctx, cluster)
	if err != nil {
		return apierror.InternalError(err)
	}
	namespaceList = auth.FilterResources(user, namespaceList)

	// Namespace() returns a namespace's own name, so this is an exact-name
	// filter here, unlike the substring match that ?search= applies below.
	requestedNamespaces := response.GetNamespacesParam(c)
	namespaceList = auth.FilterByNamespaces(namespaceList, requestedNamespaces)

	if search := response.GetSearchParam(c); search != "" {
		lower := strings.ToLower(search)
		var filtered []namespaces.Namespace
		for _, ns := range namespaceList {
			if strings.Contains(strings.ToLower(ns.Name), lower) {
				filtered = append(filtered, ns)
			}
		}
		namespaceList = filtered
	}

	sortParams := response.GetSortParams(c)

	// appCount / configCount need full maps before pagination.
	var appNamesMap map[string][]string
	var configNamesMap map[string][]string
	needCounts := sortParams.Field == "appCount" || sortParams.Field == "configCount"
	if needCounts {
		var countErr error
		appNamesMap, countErr = getAppNamesByNamespaceMap(ctx, cluster)
		if countErr != nil {
			return apierror.InternalError(countErr)
		}
		configNamesMap, countErr = getConfigurationNamesByNamespaceMap(ctx, cluster)
		if countErr != nil {
			return apierror.InternalError(countErr)
		}
		switch sortParams.Field {
		case "appCount":
			response.SortByIntKey(namespaceList, sortParams.Descending, func(ns namespaces.Namespace) int {
				return len(appNamesMap[ns.Name])
			})
		case "configCount":
			response.SortByIntKey(namespaceList, sortParams.Descending, func(ns namespaces.Namespace) int {
				return len(configNamesMap[ns.Name])
			})
		}
	} else {
		response.ApplyMetaSort(namespaceList, sortParams, response.MetaKeys[namespaces.Namespace]{
			Name:      func(ns namespaces.Namespace) string { return ns.Name },
			CreatedAt: func(ns namespaces.Namespace) time.Time { return ns.CreatedAt.Time },
		})
	}

	if page, pageSize, ok := response.GetPaginationParams(c, 1, 25); ok {
		total := len(namespaceList)
		start := (page - 1) * pageSize
		if start >= total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		pageNsList := namespaceList[start:end]

		if !needCounts {
			pageNsSet := make(map[string]bool, len(pageNsList))
			for _, ns := range pageNsList {
				pageNsSet[ns.Name] = true
			}

			allAppRefs, err := application.ListAppRefs(ctx, cluster, "")
			if err != nil {
				return apierror.InternalError(err)
			}
			appNamesMap = map[string][]string{}
			for _, ref := range allAppRefs {
				if pageNsSet[ref.Namespace] {
					appNamesMap[ref.Namespace] = append(
						appNamesMap[ref.Namespace],
						ref.Name,
					)
				}
			}

			allConfigs, err := configurations.List(ctx, cluster, "")
			if err != nil {
				return apierror.InternalError(err)
			}
			configNamesMap = map[string][]string{}
			for _, cfg := range allConfigs {
				if pageNsSet[cfg.Namespace()] {
					configNamesMap[cfg.Namespace()] = append(
						configNamesMap[cfg.Namespace()],
						cfg.Name,
					)
				}
			}
		}

		nsModels := make(models.NamespaceList, 0, len(pageNsList))
		for _, ns := range pageNsList {
			nsModels = append(nsModels, models.Namespace{
				Meta:           models.MetaLite{Name: ns.Name, CreatedAt: ns.CreatedAt},
				Apps:           appNamesMap[ns.Name],
				Configurations: configNamesMap[ns.Name],
			})
		}

		response.OKReturn(c, response.BuildPaginatedResponse(
			nsModels,
			page,
			pageSize,
			total,
		))
		return nil
	}

	if !needCounts {
		var listErr error
		appNamesMap, listErr = getAppNamesByNamespaceMap(ctx, cluster)
		if listErr != nil {
			return apierror.InternalError(listErr)
		}
		configNamesMap, listErr = getConfigurationNamesByNamespaceMap(ctx, cluster)
		if listErr != nil {
			return apierror.InternalError(listErr)
		}
	}

	nsList := make(models.NamespaceList, 0, len(namespaceList))
	for _, namespace := range namespaceList {
		nsList = append(nsList, models.Namespace{
			Meta: models.MetaLite{
				Name:      namespace.Name,
				CreatedAt: namespace.CreatedAt,
			},
			Apps:           appNamesMap[namespace.Name],
			Configurations: configNamesMap[namespace.Name],
		})
	}

	response.OKReturn(c, nsList)
	return nil
}

func getAppNamesByNamespaceMap(
	ctx context.Context,
	cluster *kubernetes.Cluster,
) (map[string][]string, error) {
	// Retrieve app references for all namespaces, and map their name by namespace
	allAppNamesMap := make(map[string][]string)

	allAppsRefs, err := application.ListAppRefs(ctx, cluster, "")
	if err != nil {
		return nil, err
	}

	for _, appRef := range allAppsRefs {
		if _, ok := allAppNamesMap[appRef.Namespace]; !ok {
			allAppNamesMap[appRef.Namespace] = make([]string, 0)
		}
		allAppNamesMap[appRef.Namespace] = append(
			allAppNamesMap[appRef.Namespace],
			appRef.Name,
		)
	}

	return allAppNamesMap, nil
}

func getConfigurationNamesByNamespaceMap(
	ctx context.Context,
	cluster *kubernetes.Cluster,
) (map[string][]string, error) {
	configurationNamesMap := make(map[string][]string)

	// Retrieve configurations for all namespaces, and map their name by namespace
	allConfigs, err := configurations.List(ctx, cluster, "")
	if err != nil {
		return nil, err
	}

	for _, config := range allConfigs {
		if _, ok := configurationNamesMap[config.Namespace()]; !ok {
			configurationNamesMap[config.Namespace()] = make([]string, 0)
		}
		configurationNamesMap[config.Namespace()] = append(
			configurationNamesMap[config.Namespace()],
			config.Name,
		)
	}

	return configurationNamesMap, nil
}
