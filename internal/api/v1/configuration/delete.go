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

package configuration

import (
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/configurationbinding"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Delete handles the API end point /namespaces/:namespace/configurations/:configuration (DELETE)
// It deletes the named configuration
func Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	// TODO: We kept this parameter for now, to not break the API.
	// As soon as the front end adapts to the array parameter below. We can
	// remove this one.
	configurationName := c.Param("configuration")

	var configurationNames []string
	configurationNames, found := c.GetQueryArray("configurations[]")
	if !found {
		configurationNames = append(configurationNames, configurationName)
	}

	username := requestctx.User(ctx).Username

	var deleteRequest models.ConfigurationDeleteRequest
	err := c.BindJSON(&deleteRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	var configurationObjects []*configurations.Configuration
	for _, cName := range configurationNames {
		configuration, err := configurations.Lookup(ctx, cluster, namespace, cName)
		if err != nil && err.Error() == "configuration not found" {
			return apierror.ConfigurationIsNotKnown(cName)
		}
		if err != nil {
			return apierror.InternalError(err)
		}

		// [SERVICE] Reject operations on configurations belonging to a service. Their manipulation
		// has to be done through service commands to keep the system in a consistent state.

		if configuration.Origin != "" {
			// [BELONG] keep in sync with same marker in the client
			return apierror.NewBadRequestErrorf("Configuration '%s' belongs to service '%s', use service requests",
				configuration.Name,
				configuration.Origin)
		}
		configurationObjects = append(configurationObjects, configuration)
	}

	// Verify that the configurations are unbound. IOW not bound to any application.
	// If they are, and automatic unbind was requested, do that.
	// Without automatic unbind such applications are reported as error.

	appConfigurationsMap := make(map[string][]string)
	allBoundApps := []string{}
	for _, cName := range configurationNames {
		boundAppNames, err := application.BoundAppsNamesFor(ctx, cluster, namespace, cName)
		if err != nil {
			return apierror.InternalError(err)
		}

		// inverted lookup map:{ appName: [configuration1, configuration2,...] }
		for _, appName := range boundAppNames {
			appConfigurationsMap[appName] = append(appConfigurationsMap[appName], cName)
		}
		allBoundApps = append(allBoundApps, boundAppNames...)
	}

	if len(allBoundApps) > 0 {
		if !deleteRequest.Unbind {
			return apierror.NewBadRequestError("bound applications exist").WithDetails(strings.Join(allBoundApps, ","))
		}

		for appName, cNames := range appConfigurationsMap {
			// Note that we reach this location only when the [SERVICE] check above
			// failed, i.e. the configuration is standalone.

			apiErr := configurationbinding.DeleteBinding(ctx, cluster, namespace, appName, username, cNames)
			if apiErr != nil {
				return apiErr
			}
		}
	}

	// Everything looks to be ok. Delete all configurations.
	for _, configuration := range configurationObjects {
		err = configuration.Delete(ctx)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	response.OKReturn(c, models.ConfigurationDeleteResponse{
		BoundApps: allBoundApps,
	})
	return nil
}
