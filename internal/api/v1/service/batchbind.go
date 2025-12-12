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
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// BatchBind handles the API endpoint /namespaces/:namespace/applications/:app/servicebindings (POST)
// It creates bindings between multiple services and the specified application in a single operation
func BatchBind(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).WithName("ServiceBatchBind")

	namespace := c.Param("namespace")
	appName := c.Param("app")

	var bindRequest models.ServiceBatchBindRequest
	err := c.BindJSON(&bindRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	if len(bindRequest.ServiceNames) == 0 {
		return apierror.NewBadRequestError("no services specified for binding")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("looking for application", "app", appName)
	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	// Collect all configuration names from all services
	allConfigurationNames := []string{}
	servicesToBind := []string{}

	// Validate all services first before making any changes
	for _, serviceName := range bindRequest.ServiceNames {
		logger.Info("validating service", "service", serviceName)

		service, apiErr := GetService(ctx, cluster, logger, namespace, serviceName)
		if apiErr != nil {
			return apiErr
		}

		apiErr = ValidateService(ctx, cluster, logger, service)
		if apiErr != nil {
			return apiErr
		}

		// Get and label the service secrets to turn them into configurations
		logger.Info("looking for secrets to label", "service", serviceName)

		configurationSecrets, err := configurations.LabelServiceSecrets(ctx, cluster, service)
		if err != nil {
			return apierror.InternalError(err)
		}

		logger.Info(fmt.Sprintf("configurationSecrets found for service %s: %+v\n", serviceName, configurationSecrets))

		// Collect configuration names from this service
		for _, secret := range configurationSecrets {
			allConfigurationNames = append(allConfigurationNames, secret.Name)
		}

		servicesToBind = append(servicesToBind, serviceName)
	}

	// Now bind all configurations at once - this triggers a SINGLE deployment
	logger.Info("binding all service configurations", "count", len(allConfigurationNames))

	_, errors := configurationbinding.CreateConfigurationBinding(
		ctx, cluster, namespace, *app, allConfigurationNames,
	)

	if errors != nil {
		return apierror.NewMultiError(errors.Errors())
	}

	// Track all service bindings
	logger.Info("recording service bindings", "services", servicesToBind)
	err = application.BoundServicesSet(ctx, cluster, app.Meta, servicesToBind, false)
	if err != nil {
		// TODO: Rewind the configuration bindings made above.
		// DANGER: This work here is not transactional :(
		return apierror.InternalError(err)
	}

	logger.Info("successfully bound services", "count", len(servicesToBind), "services", servicesToBind)

	response.OK(c)
	return nil
}

