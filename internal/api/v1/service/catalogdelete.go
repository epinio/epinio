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
	"github.com/gin-gonic/gin"
)

// CatalogDelete handles DELETE /catalogservices/:catalogservice
func CatalogDelete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)
	name := c.Param("catalogservice")

	log.Infow("delete catalog service", "name", name)
	defer log.Infow("return")

	cluster, clusterError := kubernetes.GetCluster(ctx)
	if clusterError != nil {
		return apierror.InternalError(clusterError)
	}

	kubeServiceClient, clientError := services.NewKubernetesServiceClient(cluster)
	if clientError != nil {
		return apierror.InternalError(clientError)
	}

	log.Infow("check existence", "name", name)
	exists, existsError := kubeServiceClient.CatalogServiceExists(ctx, name)
	if existsError != nil {
		return apierror.InternalError(existsError)
	}
	if !exists {
		return apierror.CatalogServiceIsNotKnown(name)
	}

	log.Infow("delete catalog service resource", "name", name)
	deleteError := kubeServiceClient.DeleteCatalogService(ctx, name)
	if deleteError != nil {
		return apierror.InternalError(deleteError)
	}

	log.Infow("catalog service deleted", "name", name)
	response.OK(c)
	return nil
}
