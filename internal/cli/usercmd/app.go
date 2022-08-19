package usercmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/pkg/errors"

	"github.com/epinio/epinio/helpers/bytes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/cli/logprinter"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
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

	if all {
		msg = c.ui.Success().WithTable("Namespace", "Name", "Created", "Status", "Routes", "Configurations", "Status Details")

		for _, app := range apps {
			if app.Workload == nil {
				msg = msg.WithTableRow(
					app.Meta.Namespace,
					app.Meta.Name,
					app.Meta.CreatedAt.String(),
					"n/a",
					"n/a",
					strings.Join(app.Configuration.Configurations, ", "),
					app.StatusMessage,
				)
			} else {
				sort.Strings(app.Workload.Routes)
				sort.Strings(app.Configuration.Configurations)
				msg = msg.WithTableRow(
					app.Meta.Namespace,
					app.Meta.Name,
					app.Meta.CreatedAt.String(),
					app.Workload.Status,
					strings.Join(app.Workload.Routes, ", "),
					strings.Join(app.Configuration.Configurations, ", "),
					app.StatusMessage,
				)
			}
		}
	} else {
		msg = c.ui.Success().WithTable("Name", "Created", "Status", "Routes", "Configurations", "Status Details")

		for _, app := range apps {
			if app.Workload == nil {
				msg = msg.WithTableRow(
					app.Meta.Name,
					app.Meta.CreatedAt.String(),
					"n/a",
					"n/a",
					strings.Join(app.Configuration.Configurations, ", "),
					app.StatusMessage,
				)
			} else {
				sort.Strings(app.Workload.Routes)
				sort.Strings(app.Configuration.Configurations)
				msg = msg.WithTableRow(
					app.Meta.Name,
					app.Meta.CreatedAt.String(),
					app.Workload.Status,
					strings.Join(app.Workload.Routes, ", "),
					strings.Join(app.Configuration.Configurations, ", "),
					app.StatusMessage,
				)
			}
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

	if err := c.printAppDetails(app); err != nil {
		return err
	}

	return c.printReplicaDetails(app)
}

// AppExport saves the named app, in the targeted namespace, to the directory.
func (c *EpinioClient) AppExport(appName string, directory string) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		WithStringValue("Target Directory", directory).
		Msg("Export application")

	if err := c.TargetOk(); err != nil {
		return err
	}

	details.Info("export application")

	err := os.MkdirAll(directory, 0700)
	if err != nil {
		return errors.Wrapf(err, "failed to create export directory '%s'", directory)
	}

	err = c.API.AppGetPart(c.Settings.Namespace, appName, "values", filepath.Join(directory, "values.yaml"))
	if err != nil {
		return err
	}

	err = c.API.AppGetPart(c.Settings.Namespace, appName, "chart", filepath.Join(directory, "app-chart.tar.gz"))
	if err != nil {
		return err
	}

	return nil
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

	details.Info("show application")

	app, err := c.API.AppShow(c.Settings.Namespace, appName)
	if err != nil {
		return err
	}

	m := models.ApplicationManifest{}
	m.Name = appName
	m.Configuration = app.Configuration
	m.Origin = app.Origin

	yaml, err := yaml.Marshal(m)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(manifestPath, yaml, 0600)
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

	return c.API.AppRestart(c.Settings.Namespace, appName)
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

	if len(appConfig.Routes) > 0 {
		msg = msg.WithStringValue("Routes", "")
		sort.Strings(appConfig.Routes)
		for i, d := range appConfig.Routes {
			msg = msg.WithStringValue(strconv.Itoa(i+1), d)
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

	return c.API.AppExec(c.Settings.Namespace, appName, instance, tty)
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

// Delete removes the named application from the cluster
func (c *EpinioClient) Delete(ctx context.Context, appname string) error {
	log := c.Log.WithName("Delete").WithValues("Application", appname)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", appname).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Deleting application...")

	if err := c.TargetOk(); err != nil {
		return err
	}

	s := c.ui.Progressf("Deleting %s in %s", appname, c.Settings.Namespace)
	defer s.Stop()

	response, err := c.API.AppDelete(c.Settings.Namespace, appname)
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

	c.ui.Success().Msg("Application deleted.")

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
			msg = msg.WithTableRow("Status", "not deployed, staging failed")
			msg = msg.WithTableRow("Last StageId", app.StageID)
		}
		msg = msg.WithTableRow("Desired Routes", "")

		if len(app.Configuration.Routes) > 0 {
			for _, route := range app.Configuration.Routes {
				msg = msg.WithTableRow("", route)
			}
		}
	}

	msg = msg.
		WithTableRow("App Chart", app.Configuration.AppChart).
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

	return nil
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
			msg = msg.WithTableRow(
				r.Name,
				strconv.FormatBool(r.Ready),
				bytes.ByteCountIEC(r.MemoryBytes),
				strconv.Itoa(int(r.MilliCPUs)),
				strconv.Itoa(int(r.Restarts)),
				time.Since(createdAt).Round(time.Second).String(),
			)
		}
		msg.Msg("Instances: ")
	}

	return nil
}

// AppRestage restage an application
func (c *EpinioClient) AppRestage(appName string) error {
	log := c.Log.WithName("AppRestage").WithValues("Namespace", c.Settings.Namespace, "Application", appName)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		Msg("Restaging application")

	if err := c.TargetOk(); err != nil {
		return err
	}

	log.V(1).Info("restaging application")

	app, err := c.API.AppShow(c.Settings.Namespace, appName)
	if err != nil {
		return err
	}

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
	_, err = c.API.StagingComplete(app.Meta.Namespace, stageID)
	return errors.Wrap(err, "waiting for staging failed")
}
