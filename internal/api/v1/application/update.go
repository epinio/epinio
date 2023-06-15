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
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/configurations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Update handles the API endpoint PATCH /namespaces/:namespace/applications/:app
func Update(c *gin.Context) apierror.APIErrors { // nolint:gocyclo // simplification defered
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	username := requestctx.User(ctx).Username
	log := requestctx.Logger(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	appRef := models.NewAppRef(appName, namespace)
	exists, err := application.Exists(ctx, cluster, appRef)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.AppIsNotKnown(appName)
	}

	// Retrieve and validate update request ...

	var updateRequest models.ApplicationUpdateRequest
	err = c.BindJSON(&updateRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	log.Info("updating app", "namespace", namespace, "app", appName, "request", updateRequest)

	if updateRequest.Instances != nil && *updateRequest.Instances < 0 {
		return apierror.NewBadRequestError("instances param should be integer equal or greater than zero")
	}

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Check if the request contains any changes. Abort early if not.

	// if there is nothing to change
	if updateRequest.Instances == nil &&
		len(updateRequest.Environment) == 0 &&
		len(updateRequest.Settings) == 0 &&
		updateRequest.Configurations == nil &&
		updateRequest.Routes == nil &&
		updateRequest.AppChart == "" {

		log.Info("updating app -- no changes")
		response.OK(c)
		return nil
	}

	if app.Workload != nil {
		// For a running application we have to validate changed custom chart values against
		// the configured app chart. It has to be done first, this ensures that there will
		// be no partial update of the application.

		// Note: If the custom chart values did not change then no validation is
		// required. It was done when the application got (re)started (pushed, or last
		// update).

		// Note: Changing the app chart is forbidden for active apps. A simple redeploy is
		// likely to run into trouble. Better to force a full re-creation (delete +
		// create/push).

		if updateRequest.AppChart != "" && updateRequest.AppChart != app.Configuration.AppChart {
			return apierror.NewBadRequestError("unable to change app chart of active application")
		}

		appChart, err := appchart.Lookup(ctx, cluster, app.Configuration.AppChart)
		if err != nil {
			return apierror.InternalError(err)
		}

		if len(updateRequest.Settings) > 0 {
			issues := application.ValidateCV(updateRequest.Settings, appChart.Settings)
			if issues != nil {
				// Treating all validation failures as internal errors.  I can't
				// find something better at the moment.

				var apiIssues []apierror.APIError
				for _, err := range issues {
					apiIssues = append(apiIssues, apierror.InternalError(err))
				}

				return apierror.NewMultiError(apiIssues)
			}
		}
	}

	// Save all changes to the relevant parts of the app resources (CRD, secrets, and the like).

	if updateRequest.AppChart != "" && updateRequest.AppChart != app.Configuration.AppChart {
		found, err := appchart.Exists(ctx, cluster, updateRequest.AppChart)
		if err != nil {
			return apierror.InternalError(err)
		}
		if !found {
			return apierror.AppChartIsNotKnown(updateRequest.AppChart)
		}

		client, err := cluster.ClientApp()
		if err != nil {
			return apierror.InternalError(err)
		}

		// Patch
		patch := fmt.Sprintf(`[{
				"op": "replace",
				"path": "/spec/chartname",
				"value": "%s" }]`,
			updateRequest.AppChart)

		log.Info("updating app", "app chart patch", patch)

		_, err = client.Namespace(app.Meta.Namespace).Patch(ctx, app.Meta.Name, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	desired, err := updateInstances(ctx, log, updateRequest.Instances, cluster, appRef)
	if err != nil {
		return apierror.InternalError(err)
	}

	if len(updateRequest.Environment) > 0 {
		log.Info("updating app", "environment", updateRequest.Environment)

		err := application.EnvironmentSet(ctx, cluster, app.Meta, updateRequest.Environment, true)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	if updateRequest.Configurations != nil {
		var okToBind []string

		log.Info("updating app", "configurations", updateRequest.Configurations)

		if len(updateRequest.Configurations) > 0 {
			for _, configurationName := range updateRequest.Configurations {
				_, err := configurations.Lookup(ctx, cluster, namespace, configurationName)
				if err != nil {
					// do not change existing configuration bindings if there is an issue
					if err.Error() == "configuration not found" {
						return apierror.ConfigurationIsNotKnown(configurationName)
					}

					return apierror.InternalError(err)
				}

				okToBind = append(okToBind, configurationName)
			}

			err = application.BoundConfigurationsSet(ctx, cluster, app.Meta, okToBind, true)
			if err != nil {
				return apierror.InternalError(err)
			}
		} else {
			// remove all bound configurations
			err = application.BoundConfigurationsSet(ctx, cluster, app.Meta, []string{}, true)
			if err != nil {
				return apierror.InternalError(err)
			}
		}
	}

	// Only update the app if routes have been set, otherwise just leave it as it is.
	// Note that an empty slice is setting routes, i.e. removing all!
	// No change is signaled by a nil slice.
	if updateRequest.Routes != nil {
		client, err := cluster.ClientApp()
		if err != nil {
			return apierror.InternalError(err)
		}

		routes := []string{}
		for _, d := range updateRequest.Routes {
			routes = append(routes, fmt.Sprintf("%q", d))
		}

		patch := fmt.Sprintf(`[{
			"op": "replace",
			"path": "/spec/routes",
			"value": [%s] }]`,
			strings.Join(routes, ","))

		log.Info("updating app", "route patch", patch)

		_, err = client.Namespace(app.Meta.Namespace).Patch(ctx, app.Meta.Name, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	// Only update the app if chart values have been set, otherwise just leave it as it is.
	if len(updateRequest.Settings) > 0 {
		client, err := cluster.ClientApp()
		if err != nil {
			return apierror.InternalError(err)
		}

		values := []string{}
		for k, v := range updateRequest.Settings {
			values = append(values, fmt.Sprintf(`%q : %q`, k, v))
		}

		patch := fmt.Sprintf(`[{
			"op": "replace",
			"path": "/spec/settings",
			"value": {%s} }]`,
			strings.Join(values, ","))

		log.Info("updating app", "settings patch", patch)

		_, err = client.Namespace(app.Meta.Namespace).Patch(ctx, app.Meta.Name, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	// With everything saved, and a workload to update, re-deploy the changed state.
	// BEWARE if the application was scaled to zero it does not seem to have a workload
	// (as there are no pods).

	if app.Workload != nil || desired > 0 {
		log.Info("updating app -- redeploy")

		_, apierr := deploy.DeployApp(ctx, cluster, app.Meta, username, "")
		if apierr != nil {
			return apierr
		}
	}

	response.OK(c)
	return nil
}

func updateInstances(ctx context.Context, log logr.Logger, instances *int32, cluster *kubernetes.Cluster, app models.AppRef) (int32, error) {
	if instances == nil {
		return 0, nil
	}

	desired := *instances
	log.Info("updating app", "desired instances", desired)

	// Save to configuration
	err := application.ScalingSet(ctx, cluster, app, desired)
	return desired, err
}
