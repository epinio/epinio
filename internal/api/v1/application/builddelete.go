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
	"encoding/json"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// DeleteBuild handles the API endpoint /namespaces/:namespace/applications/:app/build
// It clears the app's current build (staged image, blob, and staging job/s3
// leftovers) without touching the application resource itself. Refuses when
// the current build is the one actively deployed.
func DeleteBuild(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")
	name := c.Param("app")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}

	app, err := application.Lookup(ctx, cluster, namespace, name)
	if err != nil {
		return apierror.InternalError(err, "failed to look up the application")
	}
	if app == nil {
		return apierror.AppIsNotKnown("cannot delete build, application resource is missing")
	}

	// Refuse while a workload is up: after a rebuild buildstatus flips to
	// "build" even though the previous image is still running, and clearing
	// CR imageurl / deleting the staged image would break that workload.
	if app.Workload != nil {
		return apierror.NewBadRequestError(
			"cannot delete build while the application has a running workload; scale to zero or delete the app first",
		)
	}
	if app.BuildStatus == models.AppBuildStatusDeployed {
		return apierror.NewBadRequestError(
			"app is currently deployed; redeploy a different build or delete the app instead",
		)
	}
	if app.ImageURL == "" && app.StageID == "" {
		return apierror.NewBadRequestError("app has no build to delete")
	}

	appRef := app.Meta

	if _, err := application.Unstage(ctx, cluster, appRef, ""); err != nil {
		return apierror.InternalError(err, "failed to clean up staging leftovers")
	}

	if app.ImageURL != "" {
		if err := application.DeleteContainerImage(ctx, cluster, app.ImageURL); err != nil {
			// Best-effort, matching the full app-delete behavior: log and
			// continue clearing the CR fields rather than failing outright.
			log.Errorw("failed to delete container image from registry", "error", err, "image", app.ImageURL)
		}
	}

	if err := clearBuildFields(ctx, cluster, appRef); err != nil {
		return apierror.InternalError(err, "failed to clear the application's build state")
	}

	response.OK(c)
	return nil
}

// clearBuildFields merge-patches the app CR's build-related spec fields back
// to empty, the counterpart of the patch `updateApp` (stage.go) applies when
// a build is created.
func clearBuildFields(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) error {
	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	// buildstatus is an enum (build|deployed); empty string is rejected by the
	// CRD. JSON merge-patch null removes the field instead.
	specPatch := map[string]any{
		"stageid":        "",
		"blobuid":        "",
		"imageurl":       "",
		"buildstatus":    nil,
		"buildmode":      "",
		"dockerfilepath": "",
	}
	patchBody, err := json.Marshal(map[string]any{"spec": specPatch})
	if err != nil {
		return err
	}

	_, err = client.Namespace(appRef.Namespace).Patch(
		ctx,
		appRef.Name,
		types.MergePatchType,
		patchBody,
		metav1.PatchOptions{},
	)
	return err
}
