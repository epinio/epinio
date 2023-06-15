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

package configurationbinding

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// General behaviour: Internal errors (5xx) abort an action.
// Non-internal errors and warnings may be reported with it,
// however always after it. IOW an internal error is always
// the first element when reporting more than one error.

// Create handles the API endpoint /namespaces/:namespace/applications/:app/configurationbindings (POST)
// It creates a binding between the specified configuration and application
func Create(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")

	var bindRequest models.BindRequest
	err := c.BindJSON(&bindRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	if len(bindRequest.Names) == 0 {
		return apierror.NewBadRequestError("cannot bind configuration without names")
	}

	for _, configurationName := range bindRequest.Names {
		if configurationName == "" {
			return apierror.NewBadRequestError("cannot bind configuration with empty name")
		}
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	boundedConfigs, errors := CreateConfigurationBinding(ctx, cluster, namespace, *app, bindRequest.Names)
	if errors != nil {
		return errors
	}

	resp := models.BindResponse{}
	if len(boundedConfigs) > 0 {
		resp.WasBound = boundedConfigs
	}

	response.OKReturn(c, resp)
	return nil
}

func CreateConfigurationBinding(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	namespace string,
	app models.App,
	configurationNames []string,
) ([]string, apierror.APIErrors) {
	logger := requestctx.Logger(ctx).WithName("CreateConfigurationBinding")

	// Collect errors and warnings per configuration, to report as much
	// as possible while also applying as much as possible. IOW
	// even when errors are reported it is possible for some of
	// the configurations to be properly bound.

	// Take old state - See validation for use

	logger.Info("BoundConfigurationNameSet")
	oldBound, err := application.BoundConfigurationNameSet(ctx, cluster, app.Meta)
	if err != nil {
		return nil, apierror.InternalError(err)
	}

	var boundedConfigs []string
	var theIssues []apierror.APIError
	okToBind := []string{}

	// Validate existence of new configurations. Report invalid configurations as errors, later.
	// Filter out the configurations already bound, to be reported as regular response.

	logger.Info(fmt.Sprintf("configurationNames loop: %#v", configurationNames))

	for _, configurationName := range configurationNames {
		if _, ok := oldBound[configurationName]; ok {
			boundedConfigs = append(boundedConfigs, configurationName)
			continue
		}

		_, err = configurations.Lookup(ctx, cluster, namespace, configurationName)
		if err != nil {
			if err.Error() == "configuration not found" {
				theIssues = append(theIssues, apierror.ConfigurationIsNotKnown(configurationName))
				continue
			}

			theIssues = append([]apierror.APIError{apierror.InternalError(err)}, theIssues...)
			return nil, apierror.NewMultiError(theIssues)
		}

		okToBind = append(okToBind, configurationName)
	}

	logger.Info(fmt.Sprintf("okToBind: %#v", okToBind))

	if len(okToBind) > 0 {
		// Save those that were valid and not yet bound to the
		// application. Extends the set.

		logger.Info("BoundConfigurationsSet")
		err := application.BoundConfigurationsSet(ctx, cluster, app.Meta, okToBind, false)
		if err != nil {
			theIssues = append([]apierror.APIError{apierror.InternalError(err)}, theIssues...)
			return nil, apierror.NewMultiError(theIssues)
		}

		logger.Info("DeployApp")

		// Update the workload, if there is any.
		if app.Workload != nil {
			_, apierr := deploy.DeployApp(ctx, cluster, app.Meta, requestctx.User(ctx).Username, "")
			if apierr != nil {
				return nil, apierr
			}
		}
	}

	if len(theIssues) > 0 {
		return boundedConfigs, apierror.NewMultiError(theIssues)
	}

	return boundedConfigs, nil
}
