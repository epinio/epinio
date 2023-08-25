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
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/epinio/epinio/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

func Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	logger := requestctx.Logger(ctx).WithName("ServiceCreate")

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

	// Validate the chart values, if any.
	if len(createRequest.Settings) > 0 {
		issues := application.ValidateCV(createRequest.Settings, catalogService.Settings)
		if issues != nil {
			// Treating all validation failures as a bad request.
			// I can't find something better at the moment.

			var apiIssues []apierror.APIError
			for _, err := range issues {
				apiIssues = append(apiIssues, apierror.NewBadRequestError(err.Error()))
			}

			return apierror.NewMultiError(apiIssues)
		}
	}

	// Now we can (attempt to) create the desired service
	err = kubeServiceClient.Create(ctx, namespace, createRequest.Name,
		createRequest.Wait,
		createRequest.Settings,
		catalogService,
		func(ctx context.Context) error {
			return WhenFullyDeployed(ctx, cluster, logger, namespace, createRequest.Name)
		})
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OK(c)
	return nil
}

// WhenFullyDeployed is invoked when the helm chart for a service is deployed and running. At that
// point the secrets created by the service can be published as Epinio configurations.
func WhenFullyDeployed(ctx context.Context, cluster *kubernetes.Cluster, logger logr.Logger, namespace, name string) error {
	logger.Info("when fully deployed entry")

	// Called when the service is fully deployed. The context is provided as an argument
	// as it may not be the local one (closure), but a background context instead.
	// Everything else is taken from the closure.

	// Make the secrets of the newly deployed service immediately available/visible as
	// Epinio configurations.

	logger.Info("when fully deployed get service")
	service, apiErr := GetService(ctx, cluster, logger, namespace, name)
	if apiErr != nil {
		x := apiErr.(apierror.APIError)
		return fmt.Errorf("%s: %s", x.Title, x.Details)
	}

	logger.Info("when fully deployed validate service")
	apiErr = ValidateService(ctx, cluster, logger, service)
	if apiErr != nil {
		x := apiErr.(apierror.APIError)
		return fmt.Errorf("%s: %s", x.Title, x.Details)
	}

	logger.Info("when fully deployed label secrets - publish configurations")
	_, err := configurations.LabelServiceSecrets(ctx, cluster, service)
	logger.Info("when fully deployed done", "error?", err)
	return err
}
