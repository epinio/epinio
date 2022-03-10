package usercmd

import (
	"context"
	"encoding/json"
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
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/cli/logprinter"
	"github.com/epinio/epinio/internal/duration"
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
func (c *EpinioClient) Push(ctx context.Context, params PushParams) error { // nolint: gocyclo // Many ifs for view purposes
	source := params.Origin.String()
	appRef := models.AppRef{
		Name:      params.Name,
		Namespace: c.Settings.Namespace,
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
		WithStringValue("Manifest", params.Self).
		WithStringValue("Name", appRef.Name).
		WithStringValue("Source Origin", source).
		WithStringValue("Target Namespace", appRef.Namespace)
	for _, ev := range params.Configuration.Environment.List() {
		msg = msg.WithStringValue(fmt.Sprintf("Environment '%s'", ev.Name), ev.Value)
	}
	// TODO ? Make this a table for nicer alignment

	if err := c.TargetOk(); err != nil {
		return err
	}

	// Show builder, if relevant (i.e. path/git sources, not for container)
	if params.Origin.Kind != models.OriginContainer &&
		params.Staging.Builder != "" {
		msg = msg.WithStringValue("Builder", params.Staging.Builder)
	}

	if params.Configuration.Instances != nil {
		msg = msg.WithStringValue("Instances",
			strconv.Itoa(int(*params.Configuration.Instances)))
	}
	if len(params.Configuration.Services) > 0 {
		msg = msg.WithStringValue("Services",
			strings.Join(params.Configuration.Services, ", "))
	}
	if len(params.Configuration.Routes) > 0 {
		msg = msg.WithStringValue("Routes", "")
		sort.Strings(params.Configuration.Routes)
		for i, d := range params.Configuration.Routes {
			msg = msg.WithStringValue(strconv.Itoa(i+1), d)
		}
	}

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

	request := models.ApplicationCreateRequest{
		Name:          appRef.Name,
		Configuration: params.Configuration,
	}

	_, err := c.API.AppCreate(request, appRef.Namespace)
	if err != nil {
		// try to recover if it's a response type Conflict error and not a http connection error
		rerr, ok := err.(interface{ StatusCode() int })

		if !ok {
			return err
		}

		if rerr.StatusCode() != http.StatusConflict {
			return err
		}

		c.ui.Normal().Msg("Application exists, updating ...")
		details.Info("app exists conflict")

		_, err := c.API.AppUpdate(params.Configuration, appRef.Namespace, appRef.Name)
		if err != nil {
			return err
		}
	}

	// AppUpload / AppImportGit
	var blobUID string
	switch params.Origin.Kind {
	case models.OriginNone:
		return fmt.Errorf("%s", "No application origin")
	case models.OriginPath:
		c.ui.Normal().Msg("Collecting the application sources ...")

		tmpDir, tarball, err := helpers.Tar(source)
		defer func() {
			if tmpDir != "" {
				_ = os.RemoveAll(tmpDir)
			}
		}()
		if err != nil {
			return err
		}

		c.ui.Normal().Msg("Uploading application code ...")

		details.Info("upload code")
		upload, err := c.API.AppUpload(appRef.Namespace, appRef.Name, tarball)
		if err != nil {
			return err
		}
		log.V(3).Info("upload response", "response", upload)

		blobUID = upload.BlobUID

	case models.OriginGit:
		c.ui.Normal().Msg("Importing the application sources from Git ...")

		gitOrigin := params.Origin.Git
		if gitOrigin == nil {
			return errors.New("git origin is nil")
		}

		response, err := c.API.AppImportGit(appRef, *gitOrigin)
		if err != nil {
			return errors.Wrap(err, "importing git remote")
		}

		blobUID = response.BlobUID

	case models.OriginContainer:
		// Nothing to upload (nor stage)
	}

	// AppStage
	stageID := ""
	var stageResponse *models.StageResponse
	if params.Origin.Kind != models.OriginContainer {
		c.ui.Normal().Msg("Staging application with code...")

		req := models.StageRequest{
			App:          appRef,
			BlobUID:      blobUID,
			BuilderImage: params.Staging.Builder,
		}
		details.Info("staging code", "Blob", blobUID)
		stageResponse, err = c.API.AppStage(req)
		if err != nil {
			return err
		}
		stageID = stageResponse.Stage.ID
		log.V(3).Info("stage response", "response", stageResponse)

		details.Info("start tailing logs", "StageID", stageResponse.Stage.ID)
		err = c.stageLogs(details, appRef, stageResponse.Stage.ID)
		if err != nil {
			return err
		}
	}

	// AppDeploy
	c.ui.Normal().Msg("Deploying application ...")
	deployRequest := models.DeployRequest{
		App:    appRef,
		Origin: params.Origin,
	}
	// If container param is specified, then we just take it into ImageURL
	// If not, we take the one from the staging response
	if params.Origin.Kind == models.OriginContainer {
		deployRequest.ImageURL = params.Origin.Container
	} else {
		deployRequest.ImageURL = stageResponse.ImageURL
		deployRequest.Stage = models.StageRef{ID: stageID}
	}

	deployResponse, err := c.API.AppDeploy(deployRequest)
	if err != nil {
		return err
	}

	details.Info("wait for application resources")
	c.ui.ProgressNote().KeeplineUnder(1).Msg("Creating application resources")

	_, err = c.API.AppRunning(appRef)
	if err != nil {
		return errors.Wrap(err, "waiting for app failed")
	}

	routes := []string{}
	for _, d := range deployResponse.Routes {
		routes = append(routes, fmt.Sprintf("https://%s", d))
	}

	msg = c.ui.Success().
		WithStringValue("Name", appRef.Name).
		WithStringValue("Namespace", appRef.Namespace).
		WithStringValue("Builder Image", params.Staging.Builder).
		WithStringValue("Routes", "")

	if len(routes) > 0 {
		sort.Strings(routes)
		for i, r := range routes {
			msg = msg.WithStringValue(strconv.Itoa(i+1), r)
		}
	}
	msg.Msg("App is online.")

	return nil
}

func (c *EpinioClient) stageLogs(logger logr.Logger, appRef models.AppRef, stageID string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logsChan, err := c.API.AppLogs(ctx, c.Settings.Namespace, appRef.Name, stageID, true)
	if err != nil {
		c.ui.Problem().Msg(fmt.Sprintf("failed to tail logs: %s", err.Error()))
	}

	// print logs in async
	go c.printLogs(logger, logsChan)

	logger.Info("wait for job", "StageID", stageID)
	c.ui.ProgressNote().Msg("Running staging")

	// blocking function that wait until the staging is done
	_, err = c.API.StagingComplete(appRef.Namespace, stageID)
	if err != nil {
		return errors.Wrap(err, "waiting for staging failed")
	}

	return err
}

// printLogs prints the logs coming from the logsChan channel
func (c *EpinioClient) printLogs(logger logr.Logger, logsChan chan []byte) {
	printer := logprinter.LogPrinter{Tmpl: logprinter.DefaultSingleNamespaceTemplate()}

	for msg := range logsChan {
		var logLine tailer.ContainerLogLine

		if err := json.Unmarshal(msg, &logLine); err != nil {
			logger.Info("error parsing staging message", "error", err)
			return
		}

		printer.Print(logprinter.Log{
			Message:       logLine.Message,
			Namespace:     logLine.Namespace,
			PodName:       logLine.PodName,
			ContainerName: logLine.ContainerName,
		}, c.ui.ProgressNote().Compact())
	}
}
