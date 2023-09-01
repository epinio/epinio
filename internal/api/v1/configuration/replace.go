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
	"github.com/epinio/epinio/helpers/kubernetes"
	apiapp "github.com/epinio/epinio/internal/api/v1/application"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Replace handles the API endpoint PUT /namespaces/:namespace/configurations/:app
// It replaces the specified configuration.
func Replace(c *gin.Context) apierror.APIErrors { // nolint:gocyclo // simplification defered
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	configurationName := c.Param("configuration")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	configuration, err := configurations.Lookup(ctx, cluster, namespace, configurationName)
	if err != nil {
		if err.Error() == "configuration not found" {
			return apierror.ConfigurationIsNotKnown(configurationName)
		}
		return apierror.InternalError(err)
	}

	var replaceRequest models.ConfigurationReplaceRequest
	err = c.BindJSON(&replaceRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	restart, err := configurations.ReplaceConfiguration(ctx, cluster, configuration, replaceRequest)
	if err != nil {
		return apierror.InternalError(err)
	}

	if restart {
		// Determine bound apps, as candidates for restart.
		appNames, err := application.BoundAppsNamesFor(ctx, cluster, namespace, configurationName)
		if err != nil {
			return apierror.InternalError(err)
		}

		// Perform restart on the candidates which are actually running
		apiErr := apiapp.Redeploy(ctx, cluster, namespace, appNames)
		if apiErr != nil {
			return apiErr
		}
	}

	response.OK(c)
	return nil
}
