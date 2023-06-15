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
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// Unset handles the API endpoint /namespaces/:namespace/applications/:app/environment/:env (DELETE)
// It receives the namespace, application name, var name, and removes the
// variable from the application's environment.
func Unset(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)
	username := requestctx.User(ctx).Username

	namespaceName := c.Param("namespace")
	appName := c.Param("app")
	varName := c.Param("env")

	log.Info("processing environment variable removal",
		"namespace", namespaceName, "app", appName, "var", varName)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	app, err := application.Lookup(ctx, cluster, namespaceName, appName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	err = application.EnvironmentUnset(ctx, cluster, app.Meta, varName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app.Workload != nil {
		_, apierr := deploy.DeployApp(ctx, cluster, app.Meta, username, "")
		if apierr != nil {
			return apierr
		}
	}

	response.OK(c)
	return nil
}
