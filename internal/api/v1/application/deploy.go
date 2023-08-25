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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

const (
	DefaultInstances = int32(1)
	LocalRegistry    = "127.0.0.1:30500/apps"
)

// Deploy handles the API endpoint /namespaces/:namespace/applications/:app/deploy
// It uses an application chart to create the deployment, configuration and ingress (kube)
// resources for the app.
func Deploy(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	namespace := c.Param("namespace")
	name := c.Param("app")
	username := requestctx.User(ctx).Username

	req := models.DeployRequest{}
	if err := c.BindJSON(&req); err != nil {
		return apierror.NewBadRequestError(err.Error()).WithDetails("failed to unmarshal deploy request")
	}

	if name != req.App.Name {
		return apierror.NewBadRequestError("name parameter from URL does not match name param in body")
	}
	if namespace != req.App.Namespace {
		return apierror.NewBadRequestError("namespace parameter from URL does not match namespace param in body")
	}

	// validate provider reference, if actually present (git origin, and specified)
	if req.Origin.Git != nil && req.Origin.Git.Provider != "" {
		provider := req.Origin.Git.Provider
		if _, err := models.GitProviderFromString(string(provider)); err != nil {
			return apierror.NewBadRequestErrorf("bad git provider `%s`", provider)
		}

		if err := provider.ValidateURL(req.Origin.Git.URL); err != nil {
			return apierror.NewBadRequestErrorf("validating git url: `%s`", err.Error())
		}
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}

	applicationCR, err := application.Get(ctx, cluster, req.App)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierror.AppIsNotKnown("cannot deploy app, application resource is missing")
		}
		return apierror.InternalError(err, "failed to get the application resource")
	}

	err = deploy.UpdateImageURL(ctx, cluster, applicationCR, req.ImageURL)
	if err != nil {
		return apierror.InternalError(err, "failed to set application's image url")
	}

	desiredRoutes, found, err := unstructured.NestedStringSlice(applicationCR.Object, "spec", "routes")
	if err != nil {
		return apierror.InternalError(err, "failed to get the application routes")
	}
	if !found {
		// [NO-ROUTES] See other places bearing this marker for explanations.
		desiredRoutes = []string{}
	}

	apierr := validateRoutes(ctx, cluster, name, namespace, desiredRoutes)
	if apierr != nil {
		return apierr
	}

	routes, apierr := deploy.DeployApp(ctx, cluster, req.App, username, req.Stage.ID)
	if apierr != nil {
		return apierr
	}

	err = application.SetOrigin(ctx, cluster, req.App, req.Origin)
	if err != nil {
		return apierror.InternalError(err, "saving the app origin")
	}

	response.OKReturn(c, models.DeployResponse{
		Routes: routes,
	})
	return nil
}

// Redeploy does not serve a specific handler. It is used by the configuration and service
// update/replace handlers to restart the active set of the named applications. Quiescent
// applications are ignored. This is their means of forcing the applications bound to the changed
// configuration/service to pick up these changes and use them.
func Redeploy(ctx context.Context, cluster *kubernetes.Cluster, namespace string, appNames []string) apierror.APIErrors {
	username := requestctx.User(ctx).Username

	for _, appName := range appNames {
		app, err := application.Lookup(ctx, cluster, namespace, appName)
		if err != nil {
			return apierror.InternalError(err)
		}

		// Restart workload, if any
		if app.Workload != nil {
			// TODO :: This plain restart is different from all other restarts
			// (scaling, ev change, bound configurations change) ... The deployment
			// actually does not change, at all. A resource the deployment
			// references/uses changed, i.e. the configuration. We still have to
			// trigger the restart somehow, so that the pod mounting the
			// configuration remounts it for the new/changed keys.
			_, apiErr := deploy.DeployAppWithRestart(ctx, cluster, app.Meta, username, "")
			if apiErr != nil {
				return apiErr
			}
		}
	}

	return nil
}
