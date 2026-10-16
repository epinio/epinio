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
	"context"

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

	serviceList, apiErr := listServices(ctx, cluster, kubeServiceClient, namespace, getAppParam(c))
	if apiErr != nil {
		return apiErr
	}

	// The namespace scopes the binding lookup: a binding secret in another
	// namespace can never key a service listed here.
	appsOf, err := application.ServicesBoundAppsNames(ctx, cluster, namespace)
	if err != nil {
		return apierror.InternalError(err)
	}

	servicesWithApps := extendWithBoundApps(serviceList, appsOf)

	servicesWithApps = filterServices(
		servicesWithApps,
		response.GetSearchParam(c),
		getCatalogServiceParam(c),
	)

	if page, pageSize, ok := response.GetPaginationParams(c, 1, 25); ok {
		paged := response.PaginateSlice(servicesWithApps, page, pageSize)
		response.OKReturn(c, paged)
		return nil
	}

	// Backwards-compatible: return full list when no page params are set.
	response.OKReturn(c, servicesWithApps)
	return nil
}

// listServices returns the service instances to report for the namespace. With no
// `app` parameter that is every instance in the namespace. With one it is only the
// instances bound to that application, resolved from the application's binding
// secret and narrowed before the instance details are fetched.
//
// An unknown application is a 404 and not an empty list: a typo in the parameter
// must not read as "this application has no services".
func listServices(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	kubeServiceClient *services.ServiceClient,
	namespace, appName string,
) (models.ServiceList, apierror.APIErrors) {
	if appName == "" {
		serviceList, err := kubeServiceClient.ListInNamespace(ctx, namespace)
		if err != nil {
			return nil, apierror.InternalError(err)
		}

		return serviceList, nil
	}

	appRef := models.NewAppRef(appName, namespace)

	exists, err := application.Exists(ctx, cluster, appRef)
	if err != nil {
		return nil, apierror.InternalError(err)
	}
	if !exists {
		return nil, apierror.AppIsNotKnown(appName)
	}

	boundNames, err := application.BoundServiceNamesIfAny(ctx, cluster, appRef)
	if err != nil {
		return nil, apierror.InternalError(err)
	}

	serviceList, err := kubeServiceClient.ListInNamespaceByNames(ctx, namespace, boundNames)
	if err != nil {
		return nil, apierror.InternalError(err)
	}

	return serviceList, nil
}
