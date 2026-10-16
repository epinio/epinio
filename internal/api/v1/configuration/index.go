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
	"context"
	"fmt"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Index handles the API end point /namespaces/:namespace/configurations
// It returns a list of all known configuration instances
func Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	namespaceConfigurations, err := configurations.List(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	appsOf, err := application.BoundAppsNames(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	scopedConfigurations, apiErr := scopeToApp(
		ctx,
		cluster,
		namespace,
		c.Query("app"),
		namespaceConfigurations,
	)
	if apiErr != nil {
		return apiErr
	}

	// The full namespace list stays the sibling source, so that a service's other
	// configurations remain visible even when the app scope hides them.
	responseData, err := makeResponseFrom(
		ctx,
		appsOf,
		namespaceConfigurations,
		scopedConfigurations,
	)
	if err != nil {
		return apierror.InternalError(err)
	}

	if search := response.GetSearchParam(c); search != "" {
		lower := strings.ToLower(search)
		var filtered models.ConfigurationResponseList
		for _, cfg := range responseData {
			if strings.Contains(strings.ToLower(cfg.Meta.Name), lower) {
				filtered = append(filtered, cfg)
			}
		}
		responseData = filtered
	}

	if page, pageSize, ok := response.GetPaginationParams(c, 1, 25); ok {
		paged := response.PaginateSlice(responseData, page, pageSize)
		response.OKReturn(c, paged)
		return nil
	}

	// Backwards-compatible: return full list when no page params are set.
	response.OKReturn(c, responseData)
	return nil
}

// scopeToApp narrows the configuration list to the configurations bound to the
// named application. An empty application name is the unscoped case and returns
// the list unchanged. The narrowing happens here, before makeResponseFrom, so the
// per-configuration Details() lookups are paid only for the bound configurations.
//
// An unknown application is a 404 and not an empty list: a typo in the parameter
// must not read as "this application has no configurations".
func scopeToApp(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	namespace, appName string,
	configs configurations.ConfigurationList,
) (configurations.ConfigurationList, apierror.APIErrors) {
	if appName == "" {
		return configs, nil
	}

	appRef := models.NewAppRef(appName, namespace)

	exists, err := application.Exists(ctx, cluster, appRef)
	if err != nil {
		return nil, apierror.InternalError(err)
	}
	if !exists {
		return nil, apierror.AppIsNotKnown(appName)
	}

	boundNames, err := application.BoundConfigurationNamesIfAny(ctx, cluster, appRef)
	if err != nil {
		return nil, apierror.InternalError(err)
	}

	bound := map[string]struct{}{}
	for _, name := range boundNames {
		bound[name] = struct{}{}
	}

	// Never nil: a zero-match scope has to marshal to [] and not null.
	scoped := configurations.ConfigurationList{}
	for _, configuration := range configs {
		if _, ok := bound[configuration.Name]; ok {
			scoped = append(scoped, configuration)
		}
	}

	return scoped, nil
}

func makeResponse(
	ctx context.Context,
	appsOf map[application.ConfigurationKey][]string,
	configs configurations.ConfigurationList,
) (models.ConfigurationResponseList, error) {
	return makeResponseFrom(ctx, appsOf, configs, configs)
}

// makeResponseFrom builds the sibling map from allConfigs
// (so cross-page siblings remain visible) but only calls Details() for
// processConfigs. Paginated callers pass the full filtered list as allConfigs
// and only the current page as processConfigs.
func makeResponseFrom(
	ctx context.Context,
	appsOf map[application.ConfigurationKey][]string,
	allConfigs configurations.ConfigurationList,
	processConfigs configurations.ConfigurationList,
) (models.ConfigurationResponseList, error) {
	result := models.ConfigurationResponseList{}

	// Build sibling map from all configs so service-based siblings on other
	// pages are visible.
	siblingMap := map[string][]string{}
	for _, configuration := range allConfigs {
		if configuration.Origin != "" {
			key := fmt.Sprintf(
				"n%s/o%s",
				configuration.Namespace(),
				configuration.Origin,
			)
			siblingMap[key] = append(siblingMap[key], configuration.Name)
		}
	}

	// Fetch details concurrently for processConfigs only.
	results := make([]*models.ConfigurationResponse, len(processConfigs))
	group, groupCtx := errgroup.WithContext(ctx)
	for i, configuration := range processConfigs {
		i, configuration := i, configuration
		group.Go(func() error {
			configurationDetails, err := configuration.Details(groupCtx)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil // leave results[i] nil, filtered below
				}
				return err
			}

			key := application.EncodeConfigurationKey(
				configuration.Name,
				configuration.Namespace(),
			)
			appNames := appsOf[key]

			siblings := []string{}
			if configuration.Origin != "" {
				key := fmt.Sprintf(
					"n%s/o%s",
					configuration.Namespace(),
					configuration.Origin,
				)
				for _, maybeSibling := range siblingMap[key] {
					if maybeSibling != configuration.Name {
						siblings = append(siblings, maybeSibling)
					}
				}
			}

			results[i] = &models.ConfigurationResponse{
				Meta: models.ConfigurationRef{
					Meta: models.Meta{
						CreatedAt: configuration.CreatedAt,
						Name:      configuration.Name,
						Namespace: configuration.Namespace(),
					},
				},
				Configuration: models.ConfigurationShowResponse{
					Username:  configuration.User(),
					Details:   configurationDetails,
					BoundApps: appNames,
					Type:      configuration.Type,
					Origin:    configuration.Origin,
					Siblings:  siblings,
				},
			}
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return models.ConfigurationResponseList{}, err
	}

	for _, r := range results {
		if r != nil {
			result = append(result, *r)
		}
	}

	return result, nil
}
