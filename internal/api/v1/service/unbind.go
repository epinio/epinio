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
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Unbind handles the API endpoint /namespaces/:namespace/services/:service/unbind (POST)
// It removes the binding between the specified service and application
func Unbind(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).WithName("ServiceUnbind")
	username := requestctx.User(ctx).Username

	namespace := c.Param("namespace")
	serviceName := c.Param("service")

	var bindRequest models.ServiceUnbindRequest
	err := c.BindJSON(&bindRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("looking for application")
	app, err := application.Lookup(ctx, cluster, namespace, bindRequest.AppName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(bindRequest.AppName)
	}

	service, apiErr := GetService(ctx, cluster, logger, namespace, serviceName)
	if apiErr != nil {
		return apiErr
	}

	apiErr = ValidateService(ctx, cluster, logger, service)
	if apiErr != nil {
		return apiErr
	}

	// A service has one or more associated secrets containing its attributes. On
	// binding adding a specific set of labels turned these secrets into valid epinio
	// configurations. Here these configurations are simply unbound from the
	// application.

	logger.Info("looking for service secrets")

	serviceConfigurations, err := configurations.ForService(ctx, cluster, service)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("configurations", "service", service.Meta.Name, "count", len(serviceConfigurations))

	apiErr = UnbindService(ctx, cluster, logger, namespace, serviceName, app.AppRef().Name, username, serviceConfigurations)
	if apiErr != nil {
		return apiErr
	}

	response.OK(c)
	return nil
}

func UnbindService(
	ctx context.Context, cluster *kubernetes.Cluster, logger logr.Logger,
	namespace, serviceName, appName, userName string,
	serviceConfigurations []v1.Secret,
) apierror.APIErrors {
	logger.Info("unbinding service configurations", "service", serviceName, "app", appName)

	for _, secret := range serviceConfigurations {
		// TODO: Don't `helm upgrade` after each removal. Do it once.
		errors := configurationbinding.DeleteBinding(
			ctx, cluster, namespace, appName, userName, []string{secret.Name},
		)
		if errors != nil {
			return apierror.NewMultiError(errors.Errors())
		}
	}

	logger.Info("unbound service configurations")
	logger.Info("unset service/application linkage")

	appRef := models.NewAppRef(appName, namespace)
	err := application.BoundServicesUnset(ctx, cluster, appRef, serviceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Info("unbound service")
	return nil
}
