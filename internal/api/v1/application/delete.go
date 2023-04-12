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

package application

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Delete handles the API endpoint DELETE /namespaces/:namespace/applications/:app
// It removes the named application
func Delete(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	appName := c.Param("app")

	var applicationNames []string
	applicationNames, found := c.GetQueryArray("applications[]")
	if !found {
		applicationNames = append(applicationNames, appName)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	boundConfigurations := []string{}
	for _, appName := range applicationNames {
		appRef := models.NewAppRef(appName, namespace)

		found, err := application.Exists(ctx, cluster, appRef)
		if err != nil {
			return apierror.InternalError(err)
		}
		if !found {
			return apierror.AppIsNotKnown(appName)
		}

		configurations, err := application.BoundConfigurationNames(ctx, cluster, appRef)
		if err != nil {
			return apierror.InternalError(err)
		}
		boundConfigurations = append(boundConfigurations, configurations...)

		err = application.Delete(ctx, cluster, appRef)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	resp := models.ApplicationDeleteResponse{
		UnboundConfigurations: boundConfigurations,
	}

	response.OKReturn(c, resp)
	return nil
}
