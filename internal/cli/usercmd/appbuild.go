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

package usercmd

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// AppBuild builds and stores an application image without deploying it.
// Flow: validate → create/update app → upload/import → stage → wait.
func (c *EpinioClient) AppBuild(ctx context.Context, manifest models.ApplicationManifest) error {
	if manifest.Configuration.AppChart == "" {
		manifest.Configuration.AppChart = c.Settings.AppChart
	}

	source := manifest.Origin.String()
	appRef := models.AppRef{
		Meta: models.Meta{
			Name:      manifest.Name,
			Namespace: c.Settings.Namespace,
		},
	}
	log := c.Log.
		WithName("AppBuild").
		WithValues("Name", appRef.Name,
			"Namespace", appRef.Namespace,
			"Sources", source)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1)

	if manifest.Origin.Kind == models.OriginContainer {
		return errors.New("container-image origins have nothing to build; use `epinio app push` or `epinio app deploy` instead")
	}
	if manifest.Origin.Kind == models.OriginNone {
		return errors.New("no application origin")
	}

	msg := c.ui.Note().
		WithStringValue("Manifest", manifest.Self).
		WithStringValue("Name", appRef.Name).
		WithStringValue("Source Origin", source).
		WithStringValue("AppChart", manifest.Configuration.AppChart).
		WithStringValue("Target Namespace", appRef.Namespace)
	for _, ev := range manifest.Configuration.Environment.List() {
		msg = msg.WithStringValue(fmt.Sprintf("Environment '%s'", ev.Name), ev.Value)
	}

	if err := c.TargetOk(); err != nil {
		return err
	}

	if manifest.Origin.Kind != models.OriginContainer &&
		manifest.Staging.Builder != "" {
		msg = msg.WithStringValue("Builder", manifest.Staging.Builder)
	}
	if manifest.Configuration.Instances != nil {
		msg = msg.WithStringValue("Instances",
			strconv.Itoa(int(*manifest.Configuration.Instances)))
	}

	details.Info("validate app name")
	errorMsgs := validation.IsDNS1123Subdomain(appRef.Name)
	if len(errorMsgs) > 0 {
		return fmt.Errorf("%s: %s", "app name incorrect", strings.Join(errorMsgs, "\n"))
	}

	if err := validateLocalDockerfile(manifest); err != nil {
		return err
	}

	msg = routeMessage(msg, manifest.Configuration.Routes)
	msg.Msg("About to build an application (no deploy)")

	c.ui.Exclamation().
		Timeout(duration.UserAbort()).
		Msg("Hit Enter to continue or Ctrl+C to abort (build will continue automatically in 5 seconds)")

	c.ui.Normal().Msg("Create the application resource ...")

	updateRequest := models.NewApplicationUpdateRequest(manifest)
	_, err := c.API.AppCreate(models.ApplicationCreateRequest{
		Name:          appRef.Name,
		Configuration: updateRequest,
	}, appRef.Namespace)
	if err != nil {
		epinioAPIError := &client.APIError{}
		if !errors.As(err, &epinioAPIError) {
			return err
		}
		if epinioAPIError.StatusCode != http.StatusConflict {
			return err
		}

		c.ui.Normal().Msg("Application exists, updating ...")
		restart := false
		updateRequest.Restart = &restart
		_, err := c.API.AppUpdate(updateRequest, appRef.Namespace, appRef.Name)
		if err != nil {
			return err
		}
	}

	_, err = c.API.AppValidateCV(appRef.Namespace, appRef.Name)
	if err != nil {
		return err
	}

	var blobUID string
	switch manifest.Origin.Kind {
	case models.OriginPath:
		uploadedSourceBlobID, err := c.uploadSources(log, appRef, source, manifest)
		if err != nil {
			return err
		}
		blobUID = uploadedSourceBlobID
	case models.OriginGit:
		c.ui.Normal().Msg("Importing the application sources from Git ...")
		gitOrigin := manifest.Origin.Git
		if gitOrigin == nil {
			return errors.New("git origin is nil")
		}
		response, err := c.API.AppImportGit(appRef.Namespace, appRef.Name, *gitOrigin)
		if err != nil {
			return errors.Wrap(err, "importing git remote")
		}
		blobUID = response.BlobUID
		if response.Branch != "" {
			manifest.Origin.Git.Branch = response.Branch
		}
		manifest.Origin.Git.Revision = response.Revision
	default:
		return fmt.Errorf("unsupported origin for build: %s", source)
	}

	c.ui.Normal().Msg("Building application on the server ...")
	c.ui.ProgressNote().Msg("Build")

	stageReq := models.StageRequest{
		App:            appRef,
		BlobUID:        blobUID,
		BuilderImage:   manifest.Staging.Builder,
		BuildMode:      manifest.Staging.BuildMode,
		DockerfilePath: manifest.Staging.DockerfilePath,
		Origin:         manifest.Origin,
	}

	details.Info("stage start", "app", appRef)
	stageResponse, err := c.API.AppStage(stageReq)
	if err != nil {
		return errors.Wrap(err, "staging application")
	}
	if stageResponse == nil || stageResponse.Stage.ID == "" {
		return errors.New("staging did not return a stage id")
	}

	stageID := stageResponse.Stage.ID
	details.Info("start tailing logs", "StageID", stageID)
	c.stageLogs(appRef, stageID)

	s := c.ui.Progress("Waiting for build")
	defer s.Stop()

	if err := stagingWait(details, c.API, appRef.Namespace, stageID); err != nil {
		c.ui.Note().Msgf(
			"You can access the staging logs at any time, either in the UI or with the CLI using this command:\n\nepinio app logs --staging %s",
			appRef.Name)
		return errors.Wrap(err, "build failed")
	}

	c.ui.Success().
		WithStringValue("Name", appRef.Name).
		WithStringValue("Namespace", appRef.Namespace).
		WithStringValue("Image", stageResponse.ImageURL).
		WithStringValue("Stage ID", stageID).
		WithStringValue("Builder Image", manifest.Staging.Builder).
		Msg("App build stored. Deploy later with: epinio app deploy " + appRef.Name)

	return nil
}

// BuildList shows every application in the targeted namespace together with
// its current build (image and build status).
func (c *EpinioClient) BuildList(ctx context.Context) error {
	log := c.Log.WithName("BuildList").WithValues("Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")

	if err := c.TargetOk(); err != nil {
		return err
	}

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Listing application builds")

	apps, err := c.API.Apps(c.Settings.Namespace)
	if err != nil {
		return err
	}

	sort.Sort(apps)

	msg := c.ui.Success().WithTable("Name", "Build Status", "Image")
	for _, app := range apps {
		msg = msg.WithTableRow(app.Meta.Name, app.BuildStatus, app.ImageURL)
	}
	msg.Msg("Ok")

	return nil
}

// BuildShow shows the named application's current build details.
func (c *EpinioClient) BuildShow(ctx context.Context, appName string) error {
	log := c.Log.WithName("BuildShow").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		Msg("Show application build details")

	app, err := c.API.AppShow(c.Settings.Namespace, appName)
	if err != nil {
		return err
	}

	c.ui.Success().WithTable("Key", "Value").
		WithTableRow("Name", app.Meta.Name).
		WithTableRow("Build Status", app.BuildStatus).
		WithTableRow("Image", app.ImageURL).
		WithTableRow("Stage ID", app.StageID).
		WithTableRow("Blob UID", app.BlobUID).
		WithTableRow("Builder Image", app.Staging.Builder).
		WithTableRow("Build Mode", app.Staging.BuildMode).
		WithTableRow("Dockerfile Path", app.Staging.DockerfilePath).
		WithTableRow("Origin", app.Origin.String()).
		Msg("Ok")

	return nil
}

// BuildDelete clears the named application's current build (staged image and
// staging leftovers) without deleting the application itself.
func (c *EpinioClient) BuildDelete(ctx context.Context, appName string) error {
	log := c.Log.WithName("BuildDelete").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		Msg("Deleting application build...")

	_, err := c.API.AppBuildDelete(c.Settings.Namespace, appName)
	if err != nil {
		return errors.Wrap(err, "build delete failed")
	}

	c.ui.Success().
		WithStringValue("Application", appName).
		Msg("Application Build Removed.")

	return nil
}
