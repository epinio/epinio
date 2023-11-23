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
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/cli/termui"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

type PushParams struct {
	models.ApplicationManifest
}

// Push pushes an app
// * validate
// * upload
// * stage
// * (tail logs)
// * wait for staging to be done (complete or fail)
// * deploy
// * wait for app
func (c *EpinioClient) AppPush(ctx context.Context, manifest models.ApplicationManifest) error { // nolint: gocyclo // Many ifs for view purposes

	// Use settings default if user did not specify --app-chart
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
		WithName("Push").
		WithValues("Name", appRef.Name,
			"Namespace", appRef.Namespace,
			"Sources", source)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute. Visible via TRACE_LEVEL=2

	msg := c.ui.Note().
		WithStringValue("Manifest", manifest.Self). // This path is already platform-specific
		WithStringValue("Name", appRef.Name).
		WithStringValue("Source Origin", source).
		WithStringValue("AppChart", manifest.Configuration.AppChart).
		WithStringValue("Target Namespace", appRef.Namespace)
	for _, ev := range manifest.Configuration.Environment.List() {
		msg = msg.WithStringValue(fmt.Sprintf("Environment '%s'", ev.Name), ev.Value)
	}
	// TODO ? Make this a table for nicer alignment

	if err := c.TargetOk(); err != nil {
		return err
	}

	// Show builder, if relevant (i.e. path/git sources, not for container)
	if manifest.Origin.Kind != models.OriginContainer &&
		manifest.Staging.Builder != "" {
		msg = msg.WithStringValue("Builder", manifest.Staging.Builder)
	}

	if manifest.Configuration.Instances != nil {
		msg = msg.WithStringValue("Instances",
			strconv.Itoa(int(*manifest.Configuration.Instances)))
	}
	if len(manifest.Configuration.Configurations) > 0 {
		msg = msg.WithStringValue("Configurations",
			strings.Join(manifest.Configuration.Configurations, ", "))
	}

	msg = routeMessage(msg, manifest.Configuration.Routes)

	msg.Msg("About to push an application with the given setup")

	c.ui.Exclamation().
		Timeout(duration.UserAbort()).
		Msg("Hit Enter to continue or Ctrl+C to abort (deployment will continue automatically in 5 seconds)")

	details.Info("validate app name")
	errorMsgs := validation.IsDNS1123Subdomain(appRef.Name)
	if len(errorMsgs) > 0 {
		return fmt.Errorf("%s: %s", "app name incorrect", strings.Join(errorMsgs, "\n"))
	}

	// AppCreate
	c.ui.Normal().Msg("Create the application resource ...")

	updateRequest := models.NewApplicationUpdateRequest(manifest)
	_, err := c.API.AppCreate(models.ApplicationCreateRequest{
		Name:          appRef.Name,
		Configuration: updateRequest,
	}, appRef.Namespace)
	if err != nil {
		// try to recover if it's a response type Conflict error and not a http connection error
		epinioAPIError := &client.APIError{}
		if !errors.As(err, &epinioAPIError) {
			return err
		}

		if epinioAPIError.StatusCode != http.StatusConflict {
			return err
		}

		c.ui.Normal().Msg("Application exists, updating ...")
		details.Info("app exists conflict")

		// do not restart during a push
		restart := false
		updateRequest.Restart = &restart

		_, err := c.API.AppUpdate(updateRequest, appRef.Namespace, appRef.Name)
		if err != nil {
			return err
		}
	}

	// check customization
	_, err = c.API.AppValidateCV(appRef.Namespace, appRef.Name)
	if err != nil {
		return err
	}

	// AppUpload / AppImportGit
	var blobUID string
	switch manifest.Origin.Kind {
	case models.OriginNone:
		return fmt.Errorf("%s", "No application origin")
	case models.OriginPath:
		uploadedSourceBlobID, err := c.uploadSources(log, appRef, source)
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

		// validate provider reference, if actually present (git origin, and specified)
		if gitOrigin.Provider != "" {
			if _, err := models.GitProviderFromString(string(gitOrigin.Provider)); err != nil {
				return errors.Wrapf(err, "bad git provider `%s`", gitOrigin.Provider)
			}

			if err := gitOrigin.Provider.ValidateURL(gitOrigin.URL); err != nil {
				return errors.Wrap(err, "validating git url")
			}
		}

		response, err := c.API.AppImportGit(appRef.Namespace, appRef.Name, *gitOrigin)
		if err != nil {
			return errors.Wrap(err, "importing git remote")
		}

		blobUID = response.BlobUID
		// if the server resolved the branch use that in the Git Origin
		if response.Branch != "" {
			manifest.Origin.Git.Branch = response.Branch
		}
		// update the revision with the one updated by the server
		manifest.Origin.Git.Revision = response.Revision

	case models.OriginContainer:
		// Nothing to upload (nor stage)
	}

	// AppStage
	stageID := ""
	var stageResponse *models.StageResponse
	if manifest.Origin.Kind != models.OriginContainer {
		c.ui.Normal().Msg("Staging application with code...")
		c.ui.ProgressNote().Msg("Running staging")

		req := models.StageRequest{
			App:          appRef,
			BlobUID:      blobUID,
			BuilderImage: manifest.Staging.Builder,
		}
		details.Info("staging code", "Blob", blobUID)
		stageResponse, err = c.API.AppStage(req)
		if err != nil {
			return err
		}
		stageID = stageResponse.Stage.ID
		log.V(3).Info("stage response", "response", stageResponse)

		details.Info("start tailing logs", "StageID", stageResponse.Stage.ID)
		c.stageLogs(appRef, stageResponse.Stage.ID)

		details.Info("wait for job", "StageID", stageID)

		// blocking function that wait until the staging is done
		err = stagingWithRetry(log.V(1), c.API, appRef.Namespace, stageID)
		if err != nil {
			c.ui.Note().Msgf(
				"You can access the staging logs at any time, either in the UI or with the CLI using this command:\n\nepinio app logs --staging %s",
				appRef.Name)
			return errors.Wrap(err, "waiting for staging failed")
		}
	}

	// AppDeploy
	c.ui.Normal().Msg("Deploying application ...")
	deployRequest := models.DeployRequest{
		App:    appRef,
		Origin: manifest.Origin,
	}
	// If container param is specified, then we just take it into ImageURL
	// If not, we take the one from the staging response
	if manifest.Origin.Kind == models.OriginContainer {
		deployRequest.ImageURL = manifest.Origin.Container
	} else {
		deployRequest.ImageURL = stageResponse.ImageURL
		deployRequest.Stage = models.StageRef{ID: stageID}
	}

	s := c.ui.Progress("Waiting for deployment")
	defer s.Stop()

	deployResponse, err := c.API.AppDeploy(deployRequest)
	if err != nil {
		return err
	}

	details.Info("wait for application resources")
	c.ui.ProgressNote().KeeplineUnder(1).Msg("Creating application resources")

	routes := []string{}
	for _, d := range deployResponse.Routes {
		routes = append(routes, fmt.Sprintf("https://%s", d))
	}

	c.reportOK(appRef, manifest.Staging.Builder, routes)
	return nil
}

func (c *EpinioClient) uploadSources(log logr.Logger, appRef models.AppRef, source string) (string, error) {
	c.ui.Normal().Msg("Collecting the application sources ...")

	fileInfo, err := os.Stat(source)
	if err != nil {
		return "", err
	}

	archive := source

	if fileInfo.IsDir() {
		// package directory as archive/tarball
		tmpDir, tarball, err := helpers.Tar(source)
		defer os.RemoveAll(tmpDir)
		if err != nil {
			return "", err
		}
		archive = tarball
	}

	c.ui.Normal().Msg("Uploading application code ...")

	log.V(1).Info("upload code")

	// open the archive
	file, err := os.Open(archive)
	if err != nil {
		return "", errors.Wrap(err, "failed to open archive")
	}
	defer file.Close()

	upload, err := c.API.AppUpload(appRef.Namespace, appRef.Name, file)
	if err != nil {
		return "", err
	}
	log.V(3).Info("upload response", "response", upload)

	return upload.BlobUID, nil
}

func (c *EpinioClient) stageLogs(appRef models.AppRef, stageID string) {
	go func() {
		err := c.AppLogs(appRef.Name, stageID, true)
		if err != nil {
			c.ui.Problem().Msg(fmt.Sprintf("failed to tail logs: %s", err.Error()))
		}
	}()
}

func routeMessage(msg *termui.Message, routes []string) *termui.Message {
	if routes == nil {
		return msg.WithStringValue("Routes", "<<default>>")
	}
	if len(routes) == 0 {
		return msg.WithStringValue("Routes", "<<none>>")
	}

	msg = msg.WithStringValue("Routes", "")
	sort.Strings(routes)
	for i, d := range routes {
		msg = msg.WithStringValue(strconv.Itoa(i+1), d)
	}

	return msg
}

func (c *EpinioClient) reportOK(appRef models.AppRef, builder string, routes []string) {
	msg := c.ui.Success().
		WithStringValue("Name", appRef.Name).
		WithStringValue("Namespace", appRef.Namespace).
		WithStringValue("Builder Image", builder).
		WithStringValue("Routes", "")

	if len(routes) > 0 {
		sort.Strings(routes)
		for i, r := range routes {
			msg = msg.WithStringValue(strconv.Itoa(i+1), r)
		}
	}
	msg.Msg("App is online.")
}
