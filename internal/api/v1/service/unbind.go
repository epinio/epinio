// Copyright © 2021 - 2026 KRUMWARE LLC
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
	apiapp "github.com/epinio/epinio/internal/api/v1/application"
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/gin-gonic/gin"
	v1 "k8s.io/api/core/v1"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Unbind handles the API endpoint /namespaces/:namespace/services/:service/unbind (POST)
// It removes the binding between the specified service and application
func Unbind(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).With("component", "ServiceUnbind")

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

	logger.Infow("looking for application")
	app, err := application.Lookup(ctx, cluster, namespace, bindRequest.AppName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(bindRequest.AppName)
	}

	service, apiErr := GetService(ctx, cluster, namespace, serviceName)
	if apiErr != nil {
		return apiErr
	}

	apiErr = ValidateService(ctx, cluster, service)
	if apiErr != nil {
		return apiErr
	}

	// A service has one or more associated secrets containing its attributes. On
	// binding adding a specific set of labels turned these secrets into valid epinio
	// configurations. Here these configurations are simply unbound from the
	// application.

	logger.Infow("looking for service secrets")

	serviceConfigurations, err := configurations.ForService(ctx, cluster, service)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Infow("configurations", "service", service.Meta.Name, "count", len(serviceConfigurations))

	apiErr = UnbindService(
		ctx,
		cluster,
		namespace,
		serviceName,
		app.AppRef().Name,
		serviceConfigurations,
	)
	if apiErr != nil {
		return apiErr
	}

	apiapp.RespondRedeploys(c, apiapp.RedeployIfRunning(
		ctx,
		cluster,
		[]models.App{*app},
		false,
	))
	return nil
}

// UnbindService unbinds the service's configurations from the app and
// drops the service from the app's bound services. It does not redeploy;
// the caller redeploys every affected app once.
func UnbindService(
	ctx context.Context, cluster *kubernetes.Cluster,
	namespace, serviceName, appName string,
	serviceConfigurations []v1.Secret,
) apierror.APIErrors {
	logger := requestctx.Logger(ctx).With("component", "ServiceUnbind")
	logger.Infow("unbinding service configurations", "service", serviceName, "app", appName, "count", len(serviceConfigurations))

	apiErr := deleteServiceBindings(
		ctx,
		cluster,
		namespace,
		appName,
		serviceConfigurations,
		configurationbinding.DeleteBinding,
	)
	if apiErr != nil {
		return apierror.NewMultiError(apiErr.Errors())
	}

	logger.Infow("unbound service configurations")
	logger.Infow("unset service/application linkage")

	appRef := models.NewAppRef(appName, namespace)
	err := application.BoundServicesUnset(ctx, cluster, appRef, serviceName)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Infow("unbound service")
	return nil
}

// deleteServiceBindings unbinds all of a service's configurations from the
// app in one DeleteBinding call, so the app sees one helm upgrade instead of
// one per configuration. deleteBinding is a parameter so tests can count the
// calls.
func deleteServiceBindings(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	namespace string,
	appName string,
	serviceConfigurations []v1.Secret,
	deleteBinding func(
		context.Context,
		*kubernetes.Cluster,
		string,
		string,
		[]string,
	) (*models.App, apierror.APIErrors),
) apierror.APIErrors {
	configNames := make([]string, 0, len(serviceConfigurations))
	for _, secret := range serviceConfigurations {
		configNames = append(configNames, secret.Name)
	}

	if len(configNames) == 0 {
		return nil
	}

	requestctx.Logger(ctx).Infow(
		"unbinding configurations in batch",
		"configurations",
		configNames,
	)

	_, apiErr := deleteBinding(ctx, cluster, namespace, appName, configNames)
	return apiErr
}
