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

package env

import (
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/gin-gonic/gin"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Index handles the API endpoint /namespaces/:namespace/applications/:app/environment
// It receives the namespace, application name and returns the environment
// associated with that application
// Supports optional query parameter "grouped=true" to return variables separated by origin
func Index(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	namespaceName := c.Param("namespace")
	appName := c.Param("app")
	grouped := c.Query("grouped") == "true"

	helpers.Logger.Infow("returning environment", "namespace", namespaceName, "app", appName, "grouped", grouped)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	app := models.NewAppRef(appName, namespaceName)

	exists, err := application.Exists(ctx, cluster, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.AppIsNotKnown(appName)
	}

	if grouped {
		// Return variables grouped by origin
		groupedEnv, err := application.GroupedEnvironment(ctx, cluster, app)
		if err != nil {
			return apierror.InternalError(err)
		}
		response.OKReturn(c, groupedEnv)
	} else {
		// Return only user-provided environment (backward compatibility)
		environment, err := application.Environment(ctx, cluster, app)
		if err != nil {
			return apierror.InternalError(err)
		}
		response.OKReturn(c, environment)
	}

	return nil
}
