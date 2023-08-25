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
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	apiapp "github.com/epinio/epinio/internal/api/v1/application"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/services"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Update handles the API endpoint PATCH /namespaces/:namespace/services/:service
func Update(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	serviceName := c.Param("service")
	logger := requestctx.Logger(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	service, apiErr := GetService(ctx, cluster, logger, namespace, serviceName)
	if apiErr != nil {
		return apiErr
	}

	// Retrieve and validate update request ...

	var updateRequest models.ServiceUpdateRequest
	err = c.BindJSON(&updateRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	logger.Info("updating service", "namespace", namespace, "service", serviceName, "request", updateRequest)

	// Save changes to resource

	kubeServiceClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return apierror.InternalError(err)
	}

	err = kubeServiceClient.UpdateService(ctx, cluster, service, updateRequest,
		func(ctx context.Context) error {
			err := WhenFullyDeployed(ctx, cluster, logger, namespace, serviceName)
			if err != nil {
				return err
			}

			// Determine bound apps, as candidates for restart.
			appNames, err := application.ServicesBoundAppsNamesFor(ctx, cluster, namespace, serviceName)
			if err != nil {
				return err
			}

			// Perform restart on the candidates which are actually running

			apiErr = apiapp.Redeploy(ctx, cluster, namespace, appNames)
			if apiErr != nil {
				x := apiErr.(apierror.APIError)
				return fmt.Errorf("%s: %s", x.Title, x.Details)
			}

			return nil
		})
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OK(c)
	return nil
}
