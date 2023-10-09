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
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/schollz/progressbar/v3"

	"github.com/epinio/epinio/helpers/bytes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/cli/logprinter"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"k8s.io/apimachinery/pkg/util/validation"
	kubectlterm "k8s.io/kubectl/pkg/util/term"
)

// AppCreate creates an app without a workload
func (c *EpinioClient) AppCreate(appName string, appConfig models.ApplicationUpdateRequest) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	// Use settings default if user did not specify --app-chart
	if appConfig.AppChart == "" {
		appConfig.AppChart = c.Settings.AppChart
	}

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		Msg("Create application")

	details.Info("create application")

	errorMsgs := validation.IsDNS1123Subdomain(appName)
	if len(errorMsgs) > 0 {
		return fmt.Errorf("Application's name must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name', or '123-abc').")
	}

	request := models.ApplicationCreateRequest{
		Name:          appName,
		Configuration: appConfig,
	}

	_, err := c.API.AppCreate(request, c.Settings.Namespace)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Ok")
	return nil
}

// AppsMatching returns all Epinio apps having the specified prefix in their name.
func (c *EpinioClient) AppsMatching(prefix string) []string {
	log := c.Log.WithName("AppsMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")

	result := []string{}

	resp, err := c.API.AppMatch(c.Settings.Namespace, prefix)
	if err != nil {
		return result
	}

	result = resp.Names

	sort.Strings(result)

	log.Info("matches", "found", result)
	return result
}

// Apps gets all Epinio apps in the targeted namespace, or all apps in all namespaces
func (c *EpinioClient) Apps(all bool) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	msg := c.ui.Note()
	if all {
		msg.Msg("Listing all applications")
	} else {
		msg.
			WithStringValue("Namespace", c.Settings.Namespace).
			Msg("Listing applications")

		if err := c.TargetOk(); err != nil {
			return err
		}
	}

	details.Info("list applications")

	var apps models.AppList
	var err error

	if all {
		apps, err = c.API.AllApps()
	} else {
		apps, err = c.API.Apps(c.Settings.Namespace)
	}
	if err != nil {
		return err
	}

	sort.Sort(apps)

	if c.ui.JSONEnabled() {
		return c.ui.JSON(apps)
	}

	if all {
		msg = c.ui.Success().WithTable("Namespace", "Name", "Created", "Status", "Routes",
			"Configurations", "Status Details")
	} else {
		msg = c.ui.Success().WithTable("Name", "Created", "Status", "Routes",
			"Configurations", "Status Details")
	}

	for _, app := range apps {
		sort.Strings(app.Configuration.Configurations)
		configurations := strings.Join(app.Configuration.Configurations, ", ")

		var (
			status        string
			routes        string
			statusDetails string
		)

		if app.Workload == nil {
			status = "n/a"
			routes = "n/a"

			switch app.StagingStatus {
			case models.ApplicationStagingActive:
				statusDetails = "staging"
			case models.ApplicationStagingDone:
				if *app.Configuration.Instances == 0 {
					status = "0/0"
				} else {
					// staging is done, want > 0 instances, no workload
					statusDetails = "deployment failed"
				}
			case models.ApplicationStagingFailed:
				statusDetails = "staging failed"
			}
		} else {
			status = app.Workload.Status
			routes = formatRoutes(app.Workload.Routes)

			statusDetails = app.StatusMessage

			if !c.metricsOk(app.Workload) {
				if statusDetails == "" {
					statusDetails = "metrics not available"
				} else {
					statusDetails += ", metrics not available"
				}
			}
		}

		if all {
			msg = msg.WithTableRow(
				app.Meta.Namespace,
				app.Meta.Name,
				app.Meta.CreatedAt.String(),
				status,
				routes,
				configurations,
				statusDetails,
			)
		} else {
			msg = msg.WithTableRow(
				app.Meta.Name,
				app.Meta.CreatedAt.String(),
				status,
				routes,
				configurations,
				statusDetails,
			)
		}
	}

	msg.Msg("Epinio Applications:")

	return nil
}

// AppShow displays the information of the named app, in the targeted namespace
func (c *EpinioClient) AppShow(appName string) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		Msg("Show application details")

	if err := c.TargetOk(); err != nil {
		return err
	}

	details.Info("show application")

	app, err := c.API.AppShow(c.Settings.Namespace, appName)
	if err != nil {
		return err
	}

	if c.ui.JSONEnabled() {
		return c.ui.JSON(app)
	}

	if err := c.printAppDetails(app); err != nil {
		return err
	}

	return c.printReplicaDetails(app)
}

// AppExport saves the named app, in the targeted namespace, to the directory.
func (c *EpinioClient) AppExport(appName string, toRegistry bool, param models.AppExportRequest) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	msg := c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName)

	if toRegistry {
		msg.
			WithStringValue("Target Registry", param.Destination).
			Msg("Export application to registry")
	} else {
		msg.
			WithStringValue("Target Directory", param.Destination).
			Msg("Export application to local filesystem")
	}

	if err := c.TargetOk(); err != nil {
		return err
	}

	details.Info("export application")

	if toRegistry {
		// invoke server to perform the export

		_, err := c.API.AppExport(c.Settings.Namespace, appName, param)
		if err != nil {
			return err
		}

		c.ui.Success().Msg("Ok")
		return nil
	}

	// Export to local filesystem. Retrieve the parts from the server.
	directory := param.Destination

	err := os.MkdirAll(directory, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create export directory '%s'", directory)
	}

	fmt.Println()

	err = c.getPartAndWriteFile(appName, "values", filepath.Join(directory, "values.yaml"))
	if err != nil {
		return err
	}

	err = c.getPartAndWriteFile(appName, "chart", filepath.Join(directory, "app-chart.tar.gz"))
	if err != nil {
		return err
	}

	err = c.getPartAndWriteFile(appName, "image", filepath.Join(directory, "app-image.tar"))
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Ok")
	return nil
}

func (c *EpinioClient) getPartAndWriteFile(appName, part, destinationPath string) error {
	partResponse, err := c.API.AppGetPart(c.Settings.Namespace, appName, part)
	if err != nil {
		return err
	}
	defer partResponse.Data.Close()

	// Create the file
	out, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer out.Close()

	bar := progressbar.DefaultBytes(
		partResponse.ContentLength,
		fmt.Sprintf("Downloading app %s to '%s'", part, destinationPath),
	)

	// copy response body to file
	_, err = io.Copy(io.MultiWriter(out, bar), partResponse.Data)
	return err
}

// AppManifest saves the information of the named app, in the targeted namespace, into a manifest file
func (c *EpinioClient) AppManifest(appName, manifestPath string) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		WithStringValue("Destination", manifestPath).
		Msg("Save application details to manifest")

	if err := c.TargetOk(); err != nil {
		return err
	}

	details.Info("save application manifest")

	err := c.getPartAndWriteFile(appName, "manifest", manifestPath)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Saved")

	return nil
}

// AppRestart restarts an application
func (c *EpinioClient) AppRestart(appName string) error {
	log := c.Log.WithName("AppRestart").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		Msg("Restarting application")

	if err := c.TargetOk(); err != nil {
		return err
	}

	log.V(1).Info("restarting application")

	_, err := c.API.AppRestart(c.Settings.Namespace, appName)
	return err
}

// AppStageID returns the last stage id of the named app, in the targeted namespace
func (c *EpinioClient) AppStageID(appName string) (string, error) {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")

	app, err := c.API.AppShow(c.Settings.Namespace, appName)
	if err != nil {
		return "", err
	}

	return app.StageID, nil
}

// AppUpdate updates the specified running application's attributes (e.g. instances)
func (c *EpinioClient) AppUpdate(appName string, appConfig models.ApplicationUpdateRequest) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	msg := c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName)

	if appConfig.Routes != nil {
		if len(appConfig.Routes) == 0 {
			msg = msg.WithStringValue("Routes", "<clearing all>")
		} else {
			msg = msg.WithStringValue("Routes", "")
			sort.Strings(appConfig.Routes)
			for i, d := range appConfig.Routes {
				msg = msg.WithStringValue(strconv.Itoa(i+1), d)
			}
		}
	}

	msg.Msg("Update application")

	if err := c.TargetOk(); err != nil {
		return err
	}

	details.Info("update application")

	_, err := c.API.AppUpdate(appConfig, c.Settings.Namespace, appName)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Successfully updated application")

	return nil
}

// AppLogs streams the logs of all the application instances, in the targeted namespace
// If stageID is an empty string, runtime application logs are streamed. If stageID
// is set, then the matching staging logs are streamed.
// The printLogs func will print the logs from the channel until the channel will be closed.
func (c *EpinioClient) AppLogs(appName, stageID string, follow bool) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		Msg("Streaming application logs")

	if err := c.TargetOk(); err != nil {
		return err
	}

	details.Info("application logs")

	printer := logprinter.LogPrinter{Tmpl: logprinter.DefaultSingleNamespaceTemplate()}
	callback := func(logLine tailer.ContainerLogLine) {
		printer.Print(logprinter.Log{
			Message:       logLine.Message,
			Namespace:     logLine.Namespace,
			PodName:       logLine.PodName,
			ContainerName: logLine.ContainerName,
		}, c.ui.ProgressNote().Compact())
	}

	err := c.API.AppLogs(c.Settings.Namespace, appName, stageID, follow, callback)
	if err != nil {
		return err
	}

	return nil
}

func (c *EpinioClient) AppExec(ctx context.Context, appName, instance string) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")

	msg := c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName)

	if instance != "" {
		msg = msg.WithStringValue("Instance", instance)
	}

	msg.Msg("Executing a shell")

	if err := c.TargetOk(); err != nil {
		return err
	}

	tty := kubectlterm.TTY{
		In:     os.Stdin,
		Out:    os.Stdout,
		Raw:    true,
		TryDev: true,
	}

	return c.API.AppExec(ctx, c.Settings.Namespace, appName, instance, tty)
}

func (c *EpinioClient) AppPortForward(ctx context.Context, appName, instance string, address, ports []string) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")

	msg := c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName)

	if instance != "" {
		msg = msg.WithStringValue("Instance", instance)
	}

	msg.Msg("Executing port forwarding")

	if err := c.TargetOk(); err != nil {
		return err
	}

	opts := client.NewPortForwardOpts(address, ports)
	return c.API.AppPortForward(c.Settings.Namespace, appName, instance, opts)
}

// Delete removes one or more applications, specified by name
func (c *EpinioClient) AppDelete(ctx context.Context, appNames []string, all bool) error {
	if all {
		c.ui.Note().
			WithStringValue("Namespace", c.Settings.Namespace).
			Msg("Querying Applications for Deletion...")

		if err := c.TargetOk(); err != nil {
			return err
		}

		// Using the match API with a query matching everything. Avoids transmission
		// of full configuration data and having to filter client-side.
		match, err := c.API.AppMatch(c.Settings.Namespace, "")
		if err != nil {
			return err
		}
		if len(match.Names) == 0 {
			c.ui.Exclamation().Msg("No applications found to delete")
			return nil
		}

		appNames = match.Names
		sort.Strings(appNames)
	}

	namesCSV := strings.Join(appNames, ", ")
	log := c.Log.WithName("DeleteApplication").
		WithValues("Applications", namesCSV, "Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Names", namesCSV).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Deleting Applications...")

	if !all {
		if err := c.TargetOk(); err != nil {
			return err
		}
	}

	s := c.ui.Progressf("Deleting %s in %s", appNames, c.Settings.Namespace)
	defer s.Stop()

	go c.trackDeletion(appNames, func() []string {
		match, err := c.API.AppMatch(c.Settings.Namespace, "")
		if err != nil {
			return []string{}
		}
		return match.Names
	})

	response, err := c.API.AppDelete(c.Settings.Namespace, appNames)
	if err != nil {
		return err
	}

	unboundConfigurations := response.UnboundConfigurations
	if len(unboundConfigurations) > 0 {
		s.Stop()

		sort.Strings(unboundConfigurations)
		msg := c.ui.Note().WithTable("Unbound Configurations")

		for _, bonded := range unboundConfigurations {
			msg = msg.WithTableRow(bonded)
		}
		msg.Msg("")
	}

	c.ui.Success().Msg("Applications Removed.")

	return nil
}

func (c *EpinioClient) printAppDetails(app models.App) error {
	msg := c.ui.Success().WithTable("Key", "Value").
		WithTableRow("Origin", app.Origin.String()).
		WithTableRow("Created", app.Meta.CreatedAt.String())

	var createdAt time.Time
	var err error
	if app.Workload != nil {
		createdAt, err = time.Parse(time.RFC3339, app.Workload.CreatedAt)
		if err != nil {
			return err
		}
		msg = msg.WithTableRow("Status", app.Workload.Status).
			WithTableRow("Username", app.Workload.Username).
			WithTableRow("Running StageId", app.Workload.StageID).
			WithTableRow("Last StageId", app.StageID).
			WithTableRow("Age", time.Since(createdAt).Round(time.Second).String()).
			WithTableRow("Internal Route", fmt.Sprintf("%s.%s.svc.cluster.local:8080", app.Workload.Name, app.Namespace())).
			WithTableRow("Active Routes", "")

		if len(app.Workload.Routes) > 0 {
			sort.Strings(app.Workload.Routes)
			for _, r := range app.Workload.Routes {
				msg = msg.WithTableRow("", r)
			}
		}
	} else {
		if app.StageID == "" {
			msg = msg.WithTableRow("Status", "not deployed")
		} else {
			switch app.StagingStatus {
			case models.ApplicationStagingActive:
				msg = msg.WithTableRow("Status", "not deployed, staging active")
			case models.ApplicationStagingDone:
				if *app.Configuration.Instances == 0 {
					msg = msg.WithTableRow("Status", "deployed, scaled to zero")
				} else {
					// staging is done, want > 0 instances, no workload
					msg = msg.WithTableRow("Status", "staging ok, deployment failed")
				}
			case models.ApplicationStagingFailed:
				msg = msg.WithTableRow("Status", "not deployed, staging failed")
			}
			msg = msg.WithTableRow("Last StageId", app.StageID)
		}

		if len(app.Configuration.Routes) > 0 {
			msg = msg.WithTableRow("Desired Routes", "")
			for _, route := range app.Configuration.Routes {
				msg = msg.WithTableRow("", route)
			}
		} else {
			msg = msg.WithTableRow("Desired Routes", "<<none>>")
		}
	}

	msg = msg.
		WithTableRow("App Chart", app.Configuration.AppChart).
		WithTableRow("Builder Image", app.Staging.Builder).
		WithTableRow("Desired Instances", fmt.Sprintf("%d", *app.Configuration.Instances)).
		WithTableRow("Bound Configurations", strings.Join(app.Configuration.Configurations, ", ")).
		WithTableRow("Environment", "")

	if len(app.Configuration.Environment) > 0 {
		for _, ev := range app.Configuration.Environment.List() {
			msg = msg.WithTableRow("  - "+ev.Name, ev.Value)
		}
	}

	msg = msg.WithTableRow("Chart Values", "")

	if len(app.Configuration.Settings) > 0 {
		for _, cv := range app.Configuration.Settings.List() {
			msg = msg.WithTableRow("  - "+cv.Name, cv.Value)
		}
	}

	msg.Msg("Details:")

	if len(app.Configuration.Configurations) > 0 {
		c.ui.Exclamation().Msg("Attention: Migrate bound configurations derived from services to new access paths")
	}

	return nil
}

func (c *EpinioClient) metricsOk(app *models.AppDeployment) bool {
	if len(app.Replicas) == 0 {
		return true
	}

	for _, r := range app.Replicas {
		if !r.MetricsOk {
			return false
		}
	}

	return true
}

func (c *EpinioClient) printReplicaDetails(app models.App) error {
	if app.Workload == nil {
		return nil
	}

	if len(app.Workload.Replicas) > 0 {
		msg := c.ui.Success().WithTable("Name", "Ready", "Memory", "MilliCPUs", "Restarts", "Age")
		for _, r := range app.Workload.Replicas {
			createdAt, err := time.Parse(time.RFC3339, r.CreatedAt)
			if err != nil {
				return err
			}

			millis := "not available"
			memory := "not available"
			if r.MetricsOk {
				millis = strconv.Itoa(int(r.MilliCPUs))
				memory = bytes.ByteCountIEC(r.MemoryBytes)
			}

			msg = msg.WithTableRow(
				r.Name,
				strconv.FormatBool(r.Ready),
				memory,
				millis,
				strconv.Itoa(int(r.Restarts)),
				time.Since(createdAt).Round(time.Second).String(),
			)
		}
		msg.Msg("Instances: ")
	}

	return nil
}

// AppRestage restage an application
func (c *EpinioClient) AppRestage(appName string, restart bool) error {
	log := c.Log.WithName("AppRestage").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")

	app, err := c.API.AppShow(c.Settings.Namespace, appName)
	if err != nil {
		return err
	}
	if app.Workload == nil {
		// No workload
		if app.StagingStatus == models.ApplicationStagingActive {
			// Somebody already initiated staging.
			c.ui.Exclamation().Msg("Attention: Application is already staging")
			return nil
		}
		if app.Configuration.Instances != nil && *app.Configuration.Instances == 0 {
			// Scaled to zero, no workload desired -> prevent (re)start.
			restart = false
		}
	}

	m := c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName)
	if restart {
		m.Msg("Restaging and restarting application")
	} else {
		m.Msg("Restaging application")
	}

	if err := c.TargetOk(); err != nil {
		return err
	}

	log.V(1).Info("restaging application")

	if app.Origin.Kind == models.OriginContainer {
		c.ui.Note().Msg("Unable to restage container-based application")
		return nil
	}

	req := models.StageRequest{App: app.Meta}
	stageResponse, err := c.API.AppStage(req)
	if err != nil {
		return err
	}

	log.V(3).Info("stage response", "response", stageResponse)
	stageID := stageResponse.Stage.ID

	log.V(1).Info("start tailing logs", "StageID", stageID)
	c.stageLogs(app.Meta, stageID)

	log.V(1).Info("wait for job", "StageID", stageID)

	// blocking function that wait until the staging is done
	err = stagingWithRetry(log.V(1), c.API, app.Meta.Namespace, stageID)
	if err != nil {
		return err
	}

	if err != nil {
		return errors.Wrap(err, "waiting for staging failed")
	}

	if restart {
		c.ui.Note().Msg("Restarting application")
		log.V(1).Info("restarting application")

		_, err := c.API.AppRestart(c.Settings.Namespace, appName)
		return err
	}

	return nil
}

func stagingWithRetry(logger logr.Logger, apiClient APIClient, namespace, stageID string) error {
	retryCondition := func(err error) bool {
		epinioAPIError := &client.APIError{}
		// something bad happened
		if !errors.As(err, &epinioAPIError) {
			return false
		}

		// do not retry for staging failures
		errMsg := strings.ToLower(epinioAPIError.Error())
		if strings.Contains(errMsg, "failed to stage") {
			return true
		}

		// retry for any other Epinio error (StatusCode >= 400)
		logger.Info("retry because of error", "error", epinioAPIError.Error())
		return true
	}

	retryLogger := func(n uint, err error) {
		logger.Info("Retrying StagingComplete",
			"tries", fmt.Sprintf("%d/%d", n, duration.RetryMax),
			"error", err.Error(),
		)
	}

	return retry.Do(
		func() error {
			_, err := apiClient.StagingComplete(namespace, stageID)
			return err
		},
		retry.RetryIf(retryCondition),
		retry.OnRetry(retryLogger),
		retry.Delay(time.Second),
		retry.Attempts(duration.RetryMax),
	)
}

func formatRoutes(routes []string) string {
	if len(routes) > 0 {
		sort.Strings(routes)
		return strings.Join(routes, ", ")
	} else {
		return "<<none>>"
	}
}
