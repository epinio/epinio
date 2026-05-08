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
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// CatalogCreate handles POST /catalogservices
func CatalogCreate(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	log.Infow("create catalog service")
	defer log.Infow("return")

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	var createRequest models.CatalogServiceCreateRequest
	bindError := c.BindJSON(&createRequest)
	if bindError != nil {
		return apierror.NewBadRequestError(bindError.Error())
	}

	if createRequest.Name == "" {
		return apierror.NewBadRequestError("catalog service name is required")
	}
	if createRequest.HelmChart == "" {
		return apierror.NewBadRequestError("chart is required")
	}

	kubeServiceClient, clientError := services.NewKubernetesServiceClient(cluster)
	if clientError != nil {
		return apierror.InternalError(clientError)
	}

	log.Infow("check existence", "name", createRequest.Name)
	exists, existsError := kubeServiceClient.CatalogServiceExists(
		ctx,
		createRequest.Name,
	)
	if existsError != nil {
		return apierror.InternalError(existsError)
	}
	if exists {
		return apierror.CatalogServiceAlreadyKnown(createRequest.Name)
	}

	log.Infow("create catalog service resource", "name", createRequest.Name)
	_, createError := kubeServiceClient.CreateCatalogService(ctx, createRequest)
	if createError != nil {
		return apierror.InternalError(createError)
	}

	log.Infow("catalog service created", "name", createRequest.Name)
	response.Created(c)
	return nil
}
