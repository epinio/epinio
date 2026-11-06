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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/deploy"
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
// It validates the request, then answers 202 with an operation that builds
// the image when the request names no image, and deploys it with the
// application chart.
func Deploy(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	namespace := c.Param("namespace")
	name := c.Param("app")
	username := requestctx.User(ctx).Username

	req := models.DeployRequest{}
	if err := c.BindJSON(&req); err != nil {
		return apierror.NewBadRequestError(err.Error()).
			WithDetails("failed to unmarshal deploy request")
	}

	if name != req.App.Name {
		return apierror.NewBadRequestError(
			"name parameter from URL does not match name param in body",
		)
	}
	if namespace != req.App.Namespace {
		return apierror.NewBadRequestError(
			"namespace parameter from URL does not match " +
				"namespace param in body",
		)
	}
	if apiErr := validateDeployRequest(req); apiErr != nil {
		return apiErr
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(
			err,
			"failed to get access to a kube client",
		)
	}

	applicationCR, err := application.Get(ctx, cluster, req.App)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierror.AppIsNotKnown(
				"cannot deploy app, application resource is missing",
			)
		}
		return apierror.InternalError(
			err,
			"failed to get the application resource",
		)
	}

	// Without an image the deploy builds first, so it needs a source to
	// build from (blob, stored blob, or git origin). With an image it only
	// deploys, so the named build must exist and not still be running.
	var sourceErr apierror.APIErrors
	if req.Builds() {
		sourceErr = requireResolvableSource(applicationCR, req)
	} else {
		sourceErr = requireDeployableStage(ctx, cluster, req)
	}
	if sourceErr != nil {
		return sourceErr
	}

	// Reject routes another app already owns now, as the synchronous
	// deploy did; the operation checks them again before Helm runs.
	desiredRoutes, found, err := unstructured.NestedStringSlice(
		applicationCR.Object,
		"spec",
		"routes",
	)
	if err != nil {
		return apierror.InternalError(err, "failed to get the application routes")
	}
	if found {
		apiErr := validateRoutes(ctx, cluster, name, namespace, desiredRoutes)
		if apiErr != nil {
			return apiErr
		}
	}

	// Authorize the origin's git configuration now, so a forbidden one is
	// a 403 on this request rather than a failed operation.
	if apiErr := authorizeOrigin(ctx, cluster, req.Origin); apiErr != nil {
		return apiErr
	}
	if req.Builds() {
		apiErr := validateRequestedBlob(ctx, cluster, req.App, req.BlobUID)
		if apiErr != nil {
			return apiErr
		}
	}

	return startOperation(
		c,
		req.App,
		models.OperationDeploy,
		func(ctx context.Context, rep operationReporter) error {
			return runDeploy(ctx, cluster, req, username, rep)
		},
	)
}

// validateDeployRequest checks the request body without touching the
// cluster: the git provider, and the build settings when it builds.
func validateDeployRequest(req models.DeployRequest) apierror.APIErrors {
	if req.Origin.Git != nil && req.Origin.Git.Provider != "" {
		provider := req.Origin.Git.Provider
		if _, err := models.GitProviderFromString(string(provider)); err != nil {
			return apierror.NewBadRequestErrorf("bad git provider `%s`", provider)
		}

		if err := provider.ValidateURL(req.Origin.Git.URL); err != nil {
			return apierror.NewBadRequestErrorf(
				"validating git url: `%s`",
				err.Error(),
			)
		}
	}

	if !req.Builds() {
		return nil
	}

	stageReq := req.StageRequest()
	if apiErr := validateStageRequestBuildSettings(stageReq); apiErr != nil {
		return apiErr
	}

	return validateStageRequestOrigin(stageReq)
}

// requireResolvableSource rejects a building deploy that names no source
// when the app carries neither a stored blob nor a git origin to re-clone
// from. It only checks that a source can be named, so the caller gets a
// fast 400 instead of a 202 and a failing operation; staging still
// validates what it resolves.
func requireResolvableSource(
	app *unstructured.Unstructured,
	req models.DeployRequest,
) apierror.APIErrors {
	if req.BlobUID != "" ||
		(req.Origin.Kind == models.OriginGit && req.Origin.Git != nil) {
		return nil
	}

	blobUID, blobErr := application.BlobUID(app)
	if blobErr != nil {
		return apierror.InternalError(
			blobErr,
			"looking up the previous blob UID",
		)
	}
	if blobUID != "" {
		return nil
	}

	origin, originErr := application.Origin(app)
	if originErr == nil && origin.Git != nil {
		return nil
	}

	return apierror.NewBadRequestError(
		"application has no usable source: no blobuid was provided, " +
			"no stored blob is available, and it has no git origin to re-clone",
	)
}

// requireDeployableStage rejects deploying a named build that is still
// running or has failed: it would point the app at an image that does not
// exist in the registry.
func requireDeployableStage(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	req models.DeployRequest,
) apierror.APIErrors {
	if req.Stage.ID == "" {
		return nil
	}

	app, err := application.Lookup(
		ctx,
		cluster,
		req.App.Namespace,
		req.App.Name,
	)
	if err != nil {
		return apierror.InternalError(err, "failed to look up the application")
	}
	if app == nil {
		return nil
	}

	switch app.StagingStatus {
	case models.ApplicationStagingActive:
		return apierror.NewBadRequestError(
			"cannot deploy while the build is still in progress",
		)
	case models.ApplicationStagingFailed:
		return apierror.NewBadRequestError(
			"cannot deploy a failed build; rebuild the application first",
		)
	}

	return nil
}

// runDeploy is the deploy operation: build first when the request names
// no image, then deploy the image.
func runDeploy(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	req models.DeployRequest,
	username string,
	rep operationReporter,
) error {
	stageID, imageURL := req.Stage.ID, req.ImageURL

	if req.Builds() {
		// The build core saves req.Origin before staging, so a build whose
		// deploy fails still keeps its origin.
		built, err := buildAndWait(
			ctx,
			cluster,
			req.StageRequest(),
			username,
			rep,
		)
		if err != nil {
			return err
		}
		stageID, imageURL = built.Stage.ID, built.ImageURL
	} else {
		rep.Staged(stageID, imageURL)

		// No build saves the origin here. Save it only when the client sent
		// one, so an empty origin never wipes the stored one.
		if req.Origin.Kind != models.OriginNone {
			err := application.SetOrigin(ctx, cluster, req.App, req.Origin)
			if err != nil {
				return fmt.Errorf("saving the app origin: %w", err)
			}
		}
	}

	rep.Phase(models.OperationDeploying)

	result, err := deployImage(
		ctx,
		cluster,
		req.App,
		username,
		stageID,
		imageURL,
	)
	if err != nil {
		return err
	}
	rep.Deployed(result.Routes, result.Warnings)

	return nil
}

// deployImage points the app at imageURL and deploys it with the app
// chart, then records the app as deployed.
func deployImage(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	appRef models.AppRef,
	username string,
	stageID string,
	imageURL string,
) (models.DeployResponse, error) {
	applicationCR, err := application.Get(ctx, cluster, appRef)
	if err != nil {
		return models.DeployResponse{}, fmt.Errorf(
			"getting the application resource: %w",
			err,
		)
	}

	err = deploy.UpdateImageURL(ctx, cluster, applicationCR, imageURL)
	if err != nil {
		return models.DeployResponse{}, fmt.Errorf(
			"setting the application's image url: %w",
			err,
		)
	}

	desiredRoutes, found, err := unstructured.NestedStringSlice(
		applicationCR.Object,
		"spec",
		"routes",
	)
	if err != nil {
		return models.DeployResponse{}, fmt.Errorf(
			"getting the application routes: %w",
			err,
		)
	}
	if !found {
		// [NO-ROUTES] See other places bearing this marker for explanations.
		desiredRoutes = []string{}
	}

	apiErr := validateRoutes(
		ctx,
		cluster,
		appRef.Name,
		appRef.Namespace,
		desiredRoutes,
	)
	if apiErr != nil {
		return models.DeployResponse{}, apiErrorAsError(apiErr)
	}

	result, apiErr := deploy.DeployApp(ctx, cluster, appRef, username, stageID)
	if apiErr != nil {
		return models.DeployResponse{}, apiErrorAsError(apiErr)
	}

	err = application.SetBuildStatus(
		ctx,
		cluster,
		appRef,
		models.AppBuildStatusDeployed,
	)
	if err != nil {
		return models.DeployResponse{}, fmt.Errorf(
			"saving the app build status: %w",
			err,
		)
	}

	return models.DeployResponse{
		Routes:   result.Routes,
		Warnings: result.Warnings,
	}, nil
}

// Redeploy does not serve a specific handler. It is used by the
// configuration and service update/replace handlers to restart the named
// applications, so their pods remount the changed configuration. It
// starts one operation per running app and returns without waiting.
func Redeploy(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	namespace string,
	appNames []string,
) (models.OperationsResponse, apierror.APIErrors) {
	apps := []models.App{}
	for _, appName := range appNames {
		app, err := application.Lookup(ctx, cluster, namespace, appName)
		if err != nil {
			return models.OperationsResponse{}, apierror.InternalError(err)
		}
		if app != nil {
			apps = append(apps, *app)
		}
	}

	// A plain restart: the deployment does not change, a resource it
	// mounts (the configuration) did.
	return RedeployIfRunning(ctx, cluster, apps, true), nil
}
