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

package catalog

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Show handles GET /catalogservices/:catalogservice
func Show(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	serviceName := c.Param("catalogservice")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	svc, err := kubeServiceClient.GetCatalogService(ctx, serviceName)
	if err != nil {
		if k8sapierrors.IsNotFound(err) {
			return apierror.NewNotFoundError("service instance", serviceName).WithDetails(err.Error())
		}
		return apierror.InternalError(err)
	}

	inUse, err := kubeServiceClient.CatalogServicesInUse(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}
	svc.BoundServices = inUse[svc.Meta.Name]

	response.OKReturn(c, svc)
	return nil
}
