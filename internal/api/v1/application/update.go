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
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// Update handles the API endpoint PATCH /namespaces/:namespace/applications/:app
func Update(c *gin.Context) apierror.APIErrors {
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

	// Reject a busy app before changing anything, so a 409 never leaves
	// half of an update applied.
	if application.Operations.Busy(appRef) {
		return operationInProgress(appRef)
	}

	// Retrieve and validate update request ...

	var updateRequest models.ApplicationUpdateRequest
	err = c.BindJSON(&updateRequest)
	if err != nil {
		return apierror.NewBadRequestError(err.Error())
	}

	log.Infow("updating app", "namespace", namespace, "app", appName, "request", updateRequest)

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

		log.Infow("updating app -- no changes")
		response.OK(c)
		return nil
	}

	// Both stay here rather than in a helper: all three steps below read
	// chartChanging, and the rollback closure at the end needs both, after
	// applyUpdate has already patched the CRD to the new chart.
	chartChanging := updateRequest.AppChart != "" &&
		updateRequest.AppChart != app.Configuration.AppChart
	previousChart := app.Configuration.AppChart

	apierr := validateSettings(
		ctx,
		cluster,
		app,
		updateRequest,
		chartChanging,
	)
	if apierr != nil {
		return apierr
	}

	apierr = applyUpdate(
		ctx,
		cluster,
		client,
		app,
		updateRequest,
		chartChanging,
		username,
	)
	if apierr != nil {
		return apierr
	}

	// Everything is saved at this point. A nil redeploy means the update
	// is complete without touching the workload: answer 200 now. Otherwise
	// the deploy runs as an async operation and the client polls /state.
	redeploy := chooseRedeploy(
		ctx,
		cluster,
		app,
		updateRequest,
		chartChanging,
		username,
	)
	if redeploy == nil {
		response.OK(c)
		return nil
	}

	return startOperation(
		c,
		appRef,
		models.OperationUpdate,
		func(ctx context.Context, rep operationReporter) error {
			rep.Phase(models.OperationDeploying)
			apierr := redeploy(ctx)
			if apierr == nil {
				return nil
			}

			// On deploy failure after a chart change, roll the CRD chart
			// name back.
			if chartChanging {
				rollbackAppChart(
					ctx,
					cluster,
					client,
					app,
					updateRequest.AppChart,
					previousChart,
					apierr,
				)
			}

			return apiErrorAsError(apierr)
		},
	)
}

// validateSettings checks settings against the chart that will be in effect
// after this update. It runs before any mutation, so a chart switch cannot
// leave the CRD patched while the settings are incompatible.
//
// It checks when the chart changes (the request's settings, else the app's
// current ones), or when settings arrive for an app that has a workload,
// whatever that workload's status. Settings for an undeployed app with no
// chart change are not checked here.
func validateSettings(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app *models.App,
	req models.ApplicationUpdateRequest,
	chartChanging bool,
) apierror.APIErrors {
	targetChartName := app.Configuration.AppChart
	if req.AppChart != "" {
		targetChartName = req.AppChart
	}

	settingsInRequest := len(req.Settings) > 0
	// Chart changes always require a settings check (existing or
	// requested). Running apps also validate when new settings are supplied.
	if !chartChanging && (app.Workload == nil || !settingsInRequest) {
		return nil
	}

	appChart, err := appchart.LookupViaCluster(ctx, cluster, targetChartName)
	if err != nil {
		return apierror.InternalError(err)
	}
	if appChart == nil {
		return apierror.AppChartIsNotKnown(targetChartName)
	}

	settingsToValidate := req.Settings
	if chartChanging && !settingsInRequest {
		settingsToValidate = app.Configuration.Settings
	}

	if len(settingsToValidate) == 0 {
		return nil
	}

	issues := application.ValidateCV(settingsToValidate, appChart.Settings)
	if issues == nil {
		return nil
	}

	var apiIssues []apierror.APIError
	for _, err := range issues {
		apiIssues = append(apiIssues, apierror.NewBadRequestError(err.Error()))
	}

	return apierror.NewMultiError(apiIssues)
}

// applyUpdate saves every requested change to the app resources (CRD,
// secrets, and the like): chart, instances, environment, configurations,
// routes and settings, in that order. It stops at the first failure.
//
// Nothing here deploys. Each step only records the desired state; the
// running workload is unchanged until chooseRedeploy picks a deploy. An app
// in the error state therefore stays in it unless a deploy follows.
func applyUpdate(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	client dynamic.NamespaceableResourceInterface,
	app *models.App,
	req models.ApplicationUpdateRequest,
	chartChanging bool,
	username string,
) apierror.APIErrors {
	log := requestctx.Logger(ctx)
	namespace := app.Meta.Namespace
	appName := app.Meta.Name

	// update appChart (allowed for both inactive and active/running apps)
	if chartChanging {
		log.Infow("updating app", "appChart", req.AppChart)

		err := updateAppChart(
			ctx,
			cluster,
			client,
			namespace,
			appName,
			req.AppChart,
		)
		if err != nil {
			return asAPIErrors(err)
		}
	}

	// update instances
	if req.Instances != nil {
		log.Infow("updating app", "instances", *req.Instances)

		err := application.ScalingSetWithEvent(
			ctx,
			cluster,
			app.Meta,
			*req.Instances,
			username,
		)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	// update envs
	if len(req.Environment) > 0 {
		log.Infow("updating app", "environment", req.Environment)

		err := application.EnvironmentSet(
			ctx,
			cluster,
			app.Meta,
			req.Environment,
			envReplaceFlag(req.ReplaceEnv),
		)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	// update configurations
	if req.Configurations != nil {
		log.Infow("updating app", "configurations", req.Configurations)

		err := updateConfigurations(
			ctx,
			cluster,
			app.Meta,
			req.Configurations,
		)
		if err != nil {
			return asAPIErrors(err)
		}
	}

	// update routes
	if req.Routes != nil {
		log.Infow("updating app", "routes", req.Routes)

		err := updateRoutes(ctx, client, namespace, appName, req.Routes)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	// update settings only if chart values have been set, otherwise just
	// leave it as it is.
	if len(req.Settings) > 0 {
		log.Infow("updating app", "settings", req.Settings)

		err := updateChartValueSettings(
			ctx,
			client,
			namespace,
			appName,
			req.Settings,
		)
		if err != nil {
			return apierror.InternalError(err)
		}
	}

	return nil
}

// chooseRedeploy decides how the saved changes reach the workload. A nil
// result means there is nothing to deploy and the update is done.
//
// It returns nil in two situations:
//   - the caller opted out with Restart=false (--no-restart): changes,
//     including a chart switch, stay on the CRD until the next deploy.
//   - nothing should be deployed: the app has a workload that is not
//     running (error, staging, deploying), or it was never deployed and the
//     request asks for no instances.
//
// A busy app never gets here: Update answers 409 before any change.
func chooseRedeploy(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app *models.App,
	req models.ApplicationUpdateRequest,
	chartChanging bool,
	username string,
) func(context.Context) apierror.APIErrors {
	if req.Restart != nil && !*req.Restart {
		return nil
	}

	log := requestctx.Logger(ctx)
	deployApp := func(ctx context.Context) apierror.APIErrors {
		_, apierr := deploy.DeployApp(ctx, cluster, app.Meta, username, "")
		return apierr
	}

	instancesChanged := appInstancesChanged(app, req.Instances)
	chartRedeploy := chartChanging && app.Workload != nil

	switch {
	// A new replica count or chart only takes effect through a helm
	// upgrade, whatever the app's status.
	case instancesChanged || chartRedeploy:
		log.Infow(
			"updating app -- deploying",
			"chartChanging",
			chartChanging,
			"instancesChanged",
			instancesChanged,
		)
		return deployApp
	// Running app, other changes (env, settings, routes, configurations):
	// redeploy so the pods pick them up, if there is an image to run.
	case app.Workload != nil && app.Status == models.ApplicationRunning:
		log.Infow("updating app -- restarting")
		return func(ctx context.Context) apierror.APIErrors {
			return deployAppIfImageReady(ctx, cluster, app, username)
		}
	// Deployed but not running: do not revive it as a side effect of an
	// edit. The saved changes apply on its next deploy or restart.
	case app.Workload != nil:
		log.Infow(
			"updating app -- restart skipped, application not running",
			"status",
			app.Status,
		)
		return nil
	// Never deployed, and the request asks for instances: first deploy.
	case req.Instances != nil && *req.Instances > 0:
		log.Infow("updating app -- deploying")
		return deployApp
	}

	return nil
}

// rollbackAppChart puts the previous chart name back on the CRD after a
// deploy with the new chart failed. A failed rollback is logged, not
// returned: the deploy error is the one the caller reports.
func rollbackAppChart(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	client dynamic.NamespaceableResourceInterface,
	app *models.App,
	from string,
	to string,
	cause apierror.APIErrors,
) {
	log := requestctx.Logger(ctx)
	log.Errorw(
		"deploy after app chart change failed, rolling chart back",
		"from",
		from,
		"to",
		to,
		"error",
		cause,
	)

	err := updateAppChart(
		ctx,
		cluster,
		client,
		app.Meta.Namespace,
		app.Meta.Name,
		to,
	)
	if err != nil {
		log.Errorw("rolling the app chart back failed", "error", err)
	}
}

// asAPIErrors keeps an API error a helper already chose (404, 400) and
// wraps anything else as a 500.
func asAPIErrors(err error) apierror.APIErrors {
	if apiErr, ok := err.(apierror.APIError); ok {
		return apiErr
	}
	return apierror.InternalError(err)
}

// deployAppIfImageReady runs DeployApp only when the app already has a built image.
// Config and scaling changes are persisted before this call; without an imageURL,
// deployment cannot succeed and should be deferred instead of failing the save.
func deployAppIfImageReady(ctx context.Context, cluster *kubernetes.Cluster, app *models.App, username string) apierror.APIErrors {
	if app.ImageURL == "" {
		requestctx.Logger(ctx).Infow("deploy skipped because application has no image yet")
		return nil
	}

	_, apierr := deploy.DeployApp(
		ctx,
		cluster,
		app.Meta,
		username,
		"",
	)
	return apierr
}

func appInstancesChanged(app *models.App, requested *int32) bool {
	if requested == nil {
		return false
	}
	if app.Configuration.Instances == nil {
		return true
	}
	return *requested != *app.Configuration.Instances
}

func envReplaceFlag(replace *bool) bool {
	return replace != nil && *replace
}

func updateAppChart(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	client dynamic.NamespaceableResourceInterface,
	namespace string,
	appName string,
	appChart string,
) error {
	found, err := appchart.ExistsViaCluster(ctx, cluster, appChart)
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
