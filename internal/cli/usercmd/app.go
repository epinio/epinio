package usercmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/epinio/epinio/helpers/bytes"
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

// AppsMatching returns all Epinio apps having the specified prefix
// in their name.
func (c *EpinioClient) AppsMatching(prefix string) []string {
	log := c.Log.WithName("AppsMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	// Ask for all apps. Filtering is local.
	// TODO: Create new endpoint (compare `EnvMatch`) and move filtering to the server.

	apps, err := c.API.Apps(c.Settings.Namespace)
	if err != nil {
		return result
	}

	for _, app := range apps {
		details.Info("Found", "Name", app.Meta.Name)

		if strings.HasPrefix(app.Meta.Name, prefix) {
			details.Info("Matched", "Name", app.Meta.Name)
			result = append(result, app.Meta.Name)
		}
	}

	sort.Strings(result)

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
		msg = c.ui.Success().WithTable("Namespace", "Name", "Status", "Routes", "Services", "Status Details")

		for _, app := range apps {
			if app.Workload == nil {
				msg = msg.WithTableRow(
					app.Meta.Namespace,
					app.Meta.Name,
					"n/a",
					"n/a",
					strings.Join(app.Configuration.Services, ", "),
					app.StatusMessage,
				)
			} else {
				sort.Strings(app.Workload.Routes)
				sort.Strings(app.Configuration.Services)
				msg = msg.WithTableRow(
					app.Meta.Namespace,
					app.Meta.Name,
					app.Workload.Status,
					strings.Join(app.Workload.Routes, ", "),
					strings.Join(app.Configuration.Services, ", "),
					app.StatusMessage,
				)
			}
		}
	} else {
		msg = c.ui.Success().WithTable("Name", "Status", "Routes", "Services", "Status Details")

		for _, app := range apps {
			if app.Workload == nil {
				msg = msg.WithTableRow(
					app.Meta.Name,
					"n/a",
					"n/a",
					strings.Join(app.Configuration.Services, ", "),
					app.StatusMessage,
				)
			} else {
				sort.Strings(app.Workload.Routes)
				sort.Strings(app.Configuration.Services)
				msg = msg.WithTableRow(
					app.Meta.Name,
					app.Workload.Status,
					strings.Join(app.Workload.Routes, ", "),
					strings.Join(app.Configuration.Services, ", "),
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logsChan, err := c.API.AppLogs(ctx, c.Settings.Namespace, appName, stageID, follow)
	if err != nil {
		c.ui.Problem().Msg(fmt.Sprintf("failed to tail logs: %s", err.Error()))
		return err
	}

	c.printLogs(details, logsChan)

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

	unboundServices := response.UnboundServices
	if len(unboundServices) > 0 {
		s.Stop()

		sort.Strings(unboundServices)
		msg := c.ui.Note().WithTable("Unbound Services")

		for _, bonded := range unboundServices {
			msg = msg.WithTableRow(bonded)
		}
		msg.Msg("")
	}

	c.ui.Success().Msg("Application deleted.")

	return nil
}

func (c *EpinioClient) printAppDetails(app models.App) error {
	msg := c.ui.Success().WithTable("Key", "Value").
		WithTableRow("Origin", app.Origin.String())

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
		WithTableRow("Desired Instances", fmt.Sprintf("%d", *app.Configuration.Instances)).
		WithTableRow("Bound Services", strings.Join(app.Configuration.Services, ", ")).
		WithTableRow("Environment", "")

	if len(app.Configuration.Environment) > 0 {
		for _, ev := range app.Configuration.Environment.List() {
			msg = msg.WithTableRow("  - "+ev.Name, ev.Value)
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
