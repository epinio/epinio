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
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Restart handles the API endpoint POST /namespaces/:namespace/applications/:app/restart
func Restart(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	username := requestctx.User(ctx).Username

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

	if app.Configuration.Instances == nil || *app.Configuration.Instances == 0 {
		return apierror.NewAPIError("No restart possible for an application with no instances", http.StatusBadRequest)
	}

	return startOperation(c, app.Meta, models.OperationRestart,
		func(ctx context.Context, rep operationReporter) error {
			rep.Phase(models.OperationDeploying)
			return restartApp(ctx, cluster, app, username)
		})
}

// restartApp is the restart operation. It brings up a newer build when
// the app was restaged, otherwise it restarts the deployed image.
func restartApp(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app *models.App,
	username string,
) error {
	// A pending build (buildstatus "build") is either a build-only or a
	// failed stage. It must go live only through an explicit deploy, so a
	// restart keeps the currently deployed image.
	restaged := app.BuildStatus != models.AppBuildStatusBuild &&
		!strings.Contains(app.ImageURL, app.StageID)
	if restaged {
		// The stage id should be contained in the image url (as image
		// tag). As it is not found we conclude that the app was restaged,
		// and restart now has to bring this version up. Recompute the
		// image url by replacing the old tag (= old stage id) with the new
		// stage id, and save it for `DeployApp` to find.
		pieces := strings.Split(app.ImageURL, ":")
		pieces[len(pieces)-1] = app.StageID
		newImageURL := strings.Join(pieces, ":")

		applicationCR, err := application.Get(ctx, cluster, app.Meta)
		if err != nil {
			return fmt.Errorf("getting the application resource: %w", err)
		}
		err = deploy.UpdateImageURL(ctx, cluster, applicationCR, newImageURL)
		if err != nil {
			return fmt.Errorf("updating application's image url: %w", err)
		}
	}

	_, apiErr := deploy.DeployAppWithRestart(
		ctx, cluster, app.Meta, username, "",
	)
	if apiErr != nil {
		return apiErrorAsError(apiErr)
	}

	if restaged {
		err := application.SetBuildStatus(
			ctx, cluster, app.Meta, models.AppBuildStatusDeployed,
		)
		if err != nil {
			return fmt.Errorf("saving the app build status: %w", err)
		}
	}

	return nil
}
