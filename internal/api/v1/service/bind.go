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
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/configurations"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Bind handles the API endpoint /namespaces/:namespace/services/:service/bind (POST)
// It creates a binding between the specified service and application
func Bind(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx).With("component", "ServiceBind")

	namespace := c.Param("namespace")
	serviceName := c.Param("service")

	var bindRequest models.ServiceBindRequest
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

	// A service has one or more associated secrets containing its attributes. Adding
	// a specific set of labels turns these secrets into valid epinio
	// configurations. These configurations are then bound to the application.

	logger.Infow("looking for secrets to label")

	configurationSecrets, err := configurations.LabelServiceSecrets(ctx, cluster, service)
	if err != nil {
		return apierror.InternalError(err)
	}

	logger.Infow("configurationSecrets found", "secrets", configurationSecrets)

	configurationNames := []string{}
	for _, secret := range configurationSecrets {
		configurationNames = append(configurationNames, secret.Name)
	}

	logger.Infow("binding service configuration")

	_, errors := configurationbinding.CreateConfigurationBinding(
		ctx, cluster, namespace, *app, configurationNames,
	)

	if errors != nil {
		return apierror.NewMultiError(errors.Errors())
	}

	logger.Infow("binding service")

	// And track the service binding itself as well.
	okToBind := []string{serviceName}

	logger.Infow("BoundServicesSet")
	err = application.BoundServicesSet(ctx, cluster, app.Meta, okToBind, false)
	if err != nil {
		// TODO: Rewind the configuration bindings made above.
		// DANGER: This work here is not transactional :(
		return apierror.InternalError(err)
	}

	response.OK(c)
	return nil
}
