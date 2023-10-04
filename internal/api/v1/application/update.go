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
	"net/url"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
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

	client, err := cluster.ClientApp()
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

	// update appChart
	if updateRequest.AppChart != "" && updateRequest.AppChart != app.Configuration.AppChart {
		log.Info("updating app", "appChart", updateRequest.AppChart)

		err := updateAppChart(ctx, cluster, client, app.Meta.Namespace, app.Meta.Name, updateRequest.AppChart)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	// update instances
	var desired int32
	if updateRequest.Instances != nil {
		desired = *updateRequest.Instances
		log.Info("updating app", "instances", desired)

		err := application.ScalingSet(ctx, cluster, appRef, desired)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	// update envs
	if len(updateRequest.Environment) > 0 {
		log.Info("updating app", "environment", updateRequest.Environment)

		err := application.EnvironmentSet(ctx, cluster, app.Meta, updateRequest.Environment, true)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	// update configurations
	if updateRequest.Configurations != nil {
		log.Info("updating app", "configurations", updateRequest.Configurations)

		err := updateConfigurations(ctx, cluster, appRef, updateRequest.Configurations)
		if err != nil {
			if apiErr, ok := err.(apierror.APIError); ok {
				return apiErr
			}
			return apierror.InternalError(err)
		}
	}

	// update routes
	if updateRequest.Routes != nil {
		log.Info("updating app", "routes", updateRequest.Routes)

		err := updateRoutes(ctx, client, namespace, appName, updateRequest.Routes)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	// update settings only if chart values have been set, otherwise just leave it as it is.
	if len(updateRequest.Settings) > 0 {
		log.Info("updating app", "settings", updateRequest.Settings)

		err := updateChartValueSettings(ctx, client, namespace, appName, updateRequest.Settings)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	// backward compatibility: if no flag provided then restart the app
	restart := updateRequest.Restart == nil || *updateRequest.Restart
	if restart {
		if app.Workload != nil || desired > 0 {
			log.Info("updating app -- restarting")

			_, apierr := deploy.DeployApp(ctx, cluster, app.Meta, username, "")
			if apierr != nil {
				return apierr
			}
		}
	}

	response.OK(c)
	return nil
}

func updateAppChart(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	client dynamic.NamespaceableResourceInterface,
	namespace string,
	appName string,
	appChart string,
) error {
	found, err := appchart.Exists(ctx, cluster, appChart)
	if err != nil {
		return apierror.InternalError(err)
	}
	if !found {
		return apierror.AppChartIsNotKnown(appChart)
	}

	// Patch
	patch := fmt.Sprintf(`[{
				"op": "replace",
				"path": "/spec/chartname",
				"value": "%s" }]`,
		appChart)

	_, err = client.Namespace(namespace).Patch(ctx, appName, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
	if err != nil {
		return apierror.InternalError(err)
	}

	return nil
}

func updateConfigurations(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	appRef models.AppRef,
	updatedConfigurations []string,
) error {
	// if empty remove all bound configurations
	if len(updatedConfigurations) == 0 {
		return application.BoundConfigurationsSet(ctx, cluster, appRef, []string{}, true)
	}

	var okToBind []string
	for _, configurationName := range updatedConfigurations {
		_, err := configurations.Lookup(ctx, cluster, appRef.Namespace, configurationName)
		if err != nil {
			// do not change existing configuration bindings if there is an issue
			if err.Error() == "configuration not found" {
				return apierror.ConfigurationIsNotKnown(configurationName)
			}

			return apierror.InternalError(err)
		}

		okToBind = append(okToBind, configurationName)
	}

	return application.BoundConfigurationsSet(ctx, cluster, appRef, okToBind, true)
}

func updateRoutes(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	namespace string,
	appName string,
	updateRoutes []string,
) error {
	// Only update the app if routes have been set, otherwise just leave it as it is.
	// Note that an empty slice is setting routes, i.e. removing all!
	// No change is signaled by a nil slice.

	routes := []string{}
	for _, d := range updateRoutes {
		// Strip scheme prefixes, if present
		routeURL, err := url.Parse(d)
		if err != nil {
			return apierror.NewBadRequestError(err.Error()).WithDetails("failed to parse route")
		}
		if routeURL.Scheme != "" {
			d = strings.TrimPrefix(d, routeURL.Scheme+"://")
		}
		// Note %q quotes the url as required by the json patch constructed below.
		routes = append(routes, fmt.Sprintf("%q", d))
	}

	patch := fmt.Sprintf(`[{
			"op": "replace",
			"path": "/spec/routes",
			"value": [%s] }]`,
		strings.Join(routes, ","),
	)

	_, err := client.Namespace(namespace).Patch(ctx, appName, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
	return err
}

func updateChartValueSettings(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	namespace string,
	appName string,
	settings models.ChartValueSettings,
) error {
	values := []string{}
	for k, v := range settings {
		values = append(values, fmt.Sprintf(`%q : %q`, k, v))
	}

	patch := fmt.Sprintf(`[{
			"op": "replace",
			"path": "/spec/settings",
			"value": {%s} }]`,
		strings.Join(values, ","))

	_, err := client.Namespace(namespace).Patch(ctx, appName, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
	return err
}
