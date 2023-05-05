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
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/services"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

func Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	var createRequest models.ServiceCreateRequest
	err := c.BindJSON(&createRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Ensure that the service to be created does not yet exist
	service, err := kubeServiceClient.Get(ctx, namespace, createRequest.Name)
	if err != nil {
		return apierror.InternalError(err)
	}
	if service != nil {
		return apierror.ServiceAlreadyKnown(createRequest.Name)
	}

	// Ensure that the requested catalog service does exist
	catalogService, err := kubeServiceClient.GetCatalogService(ctx, createRequest.CatalogService)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return apierror.NewBadRequestError(err.Error()).
				WithDetailsf("catalog service %s not found", createRequest.CatalogService)
		}
		return apierror.InternalError(err)
	}

	// Now we can (attempt to) create the desired service
	err = kubeServiceClient.Create(ctx, namespace, createRequest.Name, createRequest.Wait, *catalogService)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OK(c)
	return nil
}
