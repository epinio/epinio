// Package clients provides Epinio CLI's main functions:
// Functionality can be split into at least:
// * the "admin client", which installs Epinio and updates configs
// * the "user client", which talks to the API server
// * the Epinio API server, which also includes the web UI server
package clients

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/cli/config"
	"github.com/epinio/epinio/internal/cli/logprinter"
	epinioapi "github.com/epinio/epinio/pkg/epinioapi/v1/client"
	"github.com/epinio/epinio/pkg/epinioapi/v1/models"

	"github.com/go-logr/logr"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/validation"
)

// EpinioClient provides functionality for talking to a
// Epinio installation on Kubernetes
type EpinioClient struct {
	Config *config.Config
	Log    logr.Logger
	ui     *termui.UI
	API    *epinioapi.Client
}

func NewEpinioClient(ctx context.Context) (*EpinioClient, error) {
	configConfig, err := config.Load()
	if err != nil {
		return nil, err
	}

	uiUI := termui.NewUI()
	apiClient, err := getEpinioAPIClient(ctx)
	if err != nil {
		return nil, err
	}
	serverURL := apiClient.URL

	logger := tracelog.NewLogger().WithName("EpinioClient").V(3)

	log := logger.WithName("New")
	log.Info("Ingress API", "url", serverURL)
	log.Info("Config API", "url", configConfig.API)

	epinioClient := &EpinioClient{
		API:    apiClient,
		ui:     uiUI,
		Config: configConfig,
		Log:    logger,
	}
	return epinioClient, nil
}

// EnvList displays a table of all environment variables and their
// values for the named application.
func (c *EpinioClient) EnvList(ctx context.Context, appName string) error {
	log := c.Log.WithName("EnvList")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Config.Org).
		WithStringValue("Application", appName).
		Msg("Show Application Environment")

	if err := c.TargetOk(); err != nil {
		return err
	}

	eVariables, err := c.API.EnvList(c.Config.Org, appName)
	if err != nil {
		return err
	}

	msg := c.ui.Success().WithTable("Variable", "Value")

	sort.Sort(eVariables)
	for _, ev := range eVariables {
		msg = msg.WithTableRow(ev.Name, ev.Value)
	}

	msg.Msg("Ok")
	return nil
}

// EnvSet adds or modifies the specified environment variable in the
// named application, with the given value. A workload is restarted.
func (c *EpinioClient) EnvSet(ctx context.Context, appName, envName, envValue string) error {
	log := c.Log.WithName("Env")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Config.Org).
		WithStringValue("Application", appName).
		WithStringValue("Variable", envName).
		WithStringValue("Value", envValue).
		Msg("Extend or modify application environment")

	if err := c.TargetOk(); err != nil {
		return err
	}

	request := models.EnvVariableList{
		models.EnvVariable{
			Name:  envName,
			Value: envValue,
		},
	}

	_, err := c.API.EnvSet(request, c.Config.Org, appName)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("OK")
	return nil
}

// EnvShow shows the value of the specified environment variable in
// the named application.
func (c *EpinioClient) EnvShow(ctx context.Context, appName, envName string) error {
	log := c.Log.WithName("Env")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Config.Org).
		WithStringValue("Application", appName).
		WithStringValue("Variable", envName).
		Msg("Show application environment variable")

	if err := c.TargetOk(); err != nil {
		return err
	}

	eVariable, err := c.API.EnvShow(c.Config.Org, appName, envName)
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Value", eVariable.Value).
		Msg("OK")

	return nil
}

// EnvUnset removes the specified environment variable from the named
// application. A workload is restarted.
func (c *EpinioClient) EnvUnset(ctx context.Context, appName, envName string) error {
	log := c.Log.WithName("Env")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Config.Org).
		WithStringValue("Application", appName).
		WithStringValue("Variable", envName).
		Msg("Remove from application environment")

	if err := c.TargetOk(); err != nil {
		return err
	}

	_, err := c.API.EnvUnset(c.Config.Org, appName, envName)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("OK")

	return nil
}

// EnvMatching retrieves all environment variables in the cluster, for
// the specified application, and the given prefix
func (c *EpinioClient) EnvMatching(ctx context.Context, appName, prefix string) []string {
	log := c.Log.WithName("Env")
	log.Info("start")
	defer log.Info("return")

	resp, err := c.API.EnvMatch(c.Config.Org, appName, prefix)
	if err != nil {
		// TODO log that we dropped an error
		return []string{}
	}

	return resp.Names
}

// ServicePlans gets all service classes in the cluster, for the
// specified class
func (c *EpinioClient) ServicePlans(serviceClassName string) error {
	log := c.Log.WithName("ServicePlans").WithValues("ServiceClass", serviceClassName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().Msg("Listing service plans")

	servicePlans, err := c.API.ServicePlans(serviceClassName)
	if err != nil {
		return err
	}

	details.Info("list service plans")

	sort.Sort(servicePlans)
	msg := c.ui.Success().WithTable("Plan", "Free", "Description")
	for _, sp := range servicePlans {
		var isFree string
		if sp.Free {
			isFree = "yes"
		} else {
			isFree = "no"
		}
		msg = msg.WithTableRow(sp.Name, isFree, sp.Description)
	}
	msg.Msg("Epinio Service Plans:")

	return nil
}

// ServicePlanMatching gets all service plans in the cluster, for the
// specified class, and the given prefix
func (c *EpinioClient) ServicePlanMatching(ctx context.Context, serviceClassName, prefix string) []string {
	log := c.Log.WithName("ServicePlans").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	// Ask for all service plans of a service class. Filtering is local.
	// TODO: Create new endpoint (compare `EnvMatch`) and move filtering to the server.

	servicePlans, err := c.API.ServicePlans(serviceClassName)
	if err != nil {
		return result
	}

	for _, sp := range servicePlans {
		details.Info("Found", "Name", sp.Name)
		if strings.HasPrefix(sp.Name, prefix) {
			details.Info("Matched", "Name", sp.Name)
			result = append(result, sp.Name)
		}
	}

	return result
}

// ServiceClassMatching returns all service classes in the cluster which have the specified prefix in their name
func (c *EpinioClient) ServiceClassMatching(ctx context.Context, prefix string) []string {
	log := c.Log.WithName("ServiceClasses").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	// Ask for all service classes. Filtering is local.
	// TODO: Create new endpoint (compare `EnvMatch`) and move filtering to the server.

	serviceClasses, err := c.API.ServiceClasses()
	if err != nil {
		return result
	}

	details.Info("Filtering")
	for _, sc := range serviceClasses {
		details.Info("Found", "Name", sc.Name)
		if strings.HasPrefix(sc.Name, prefix) {
			details.Info("Matched", "Name", sc.Name)
			result = append(result, sc.Name)
		}
	}

	return result
}

// ServiceClasses gets all service classes in the cluster
func (c *EpinioClient) ServiceClasses() error {
	log := c.Log.WithName("ServiceClasses")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		Msg("Listing service classes")

	serviceClasses, err := c.API.ServiceClasses()
	if err != nil {
		return err
	}

	details.Info("list service classes")

	sort.Sort(serviceClasses)
	msg := c.ui.Success().WithTable("Name", "Description", "Broker")
	for _, sc := range serviceClasses {
		msg = msg.WithTableRow(sc.Name, sc.Description, sc.Broker)
	}
	msg.Msg("Epinio Service Classes:")

	return nil
}

// Services gets all Epinio services in the targeted org
func (c *EpinioClient) Services() error {
	log := c.Log.WithName("Services").WithValues("Namespace", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Namespace", c.Config.Org).
		Msg("Listing services")

	if err := c.TargetOk(); err != nil {
		return err
	}

	details.Info("list services")

	response, err := c.API.Services(c.Config.Org)
	if err != nil {
		return err
	}

	details.Info("list services")

	sort.Sort(response)
	msg := c.ui.Success().WithTable("Name", "Applications")

	details.Info("list services")
	for _, service := range response {
		msg = msg.WithTableRow(service.Name, strings.Join(service.BoundApps, ", "))
	}
	msg.Msg("Epinio Services:")

	return nil
}

// ServiceMatching returns all Epinio services having the specified prefix
// in their name.
func (c *EpinioClient) ServiceMatching(ctx context.Context, prefix string) []string {
	log := c.Log.WithName("ServiceMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	// Ask for all services. Filtering is local.
	// TODO: Create new endpoint (compare `EnvMatch`) and move filtering to the server.

	response, err := c.API.Services(c.Config.Org)
	if err != nil {
		return result
	}

	for _, s := range response {
		service := s.Name
		details.Info("Found", "Name", service)
		if strings.HasPrefix(service, prefix) {
			details.Info("Matched", "Name", service)
			result = append(result, service)
		}
	}

	return result
}

// BindService attaches a service specified by name to the named application,
// both in the targeted organization.
func (c *EpinioClient) BindService(serviceName, appName string) error {
	log := c.Log.WithName("Bind Service To Application").
		WithValues("Name", serviceName, "Application", appName, "Namespace", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Namespace", c.Config.Org).
		Msg("Bind Service")

	if err := c.TargetOk(); err != nil {
		return err
	}

	request := models.BindRequest{
		Names: []string{serviceName},
	}

	br, err := c.API.ServiceBindingCreate(request, c.Config.Org, appName)
	if err != nil {
		return err
	}

	if len(br.WasBound) > 0 {
		c.ui.Success().
			WithStringValue("Service", serviceName).
			WithStringValue("Application", appName).
			WithStringValue("Namespace", c.Config.Org).
			Msg("Service Already Bound to Application.")

		return nil
	}

	c.ui.Success().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Namespace", c.Config.Org).
		Msg("Service Bound to Application.")
	return nil
}

// UnbindService detaches the service specified by name from the named
// application, both in the targeted organization.
func (c *EpinioClient) UnbindService(serviceName, appName string) error {
	log := c.Log.WithName("Unbind Service").
		WithValues("Name", serviceName, "Application", appName, "Namespace", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Namespace", c.Config.Org).
		Msg("Unbind Service from Application")

	if err := c.TargetOk(); err != nil {
		return err
	}

	_, err := c.API.ServiceBindingDelete(c.Config.Org, appName, serviceName)
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Namespace", c.Config.Org).
		Msg("Service Detached From Application.")
	return nil
}

// DeleteService deletes a service specified by name
func (c *EpinioClient) DeleteService(name string, unbind bool) error {
	log := c.Log.WithName("Delete Service").
		WithValues("Name", name, "Namespace", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Config.Org).
		Msg("Delete Service")

	if err := c.TargetOk(); err != nil {
		return err
	}

	request := models.DeleteRequest{
		Unbind: unbind,
	}

	deleteResponse, err := c.API.ServiceDelete(request, c.Config.Org, name,
		func(response *http.Response, bodyBytes []byte, err error) error {
			// nothing special for internal errors and the like
			if response.StatusCode != http.StatusBadRequest {
				return err
			}

			// A bad request happens when the service is
			// still bound to one or more applications,
			// and the response contains an array of their
			// names.

			var apiError api.ErrorResponse
			if err := json.Unmarshal(bodyBytes, &apiError); err != nil {
				return err
			}

			bound := strings.Split(apiError.Errors[0].Details, ",")

			sort.Strings(bound)
			msg := c.ui.Exclamation().WithTable("Bound Applications")

			for _, app := range bound {
				msg = msg.WithTableRow(app)
			}

			msg.Msg("Unable to delete service. It is still used by")
			c.ui.Exclamation().Compact().Msg("Use --unbind to force the issue")

			return errors.New(http.StatusText(response.StatusCode))
		})
	if err != nil {
		return err
	}

	if len(deleteResponse.BoundApps) > 0 {
		sort.Strings(deleteResponse.BoundApps)
		msg := c.ui.Note().WithTable("Previously Bound To")

		for _, app := range deleteResponse.BoundApps {
			msg = msg.WithTableRow(app)
		}

		msg.Msg("")
	}

	c.ui.Success().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Config.Org).
		Msg("Service Removed.")
	return nil
}

// CreateService creates a service specified by name, class, plan, and optional key/value dictionary
// TODO: Allow underscores in service names (right now they fail because of kubernetes naming rules for secrets)
func (c *EpinioClient) CreateService(name, class, plan string, data string, waitForProvision bool) error {
	log := c.Log.WithName("Create Service").
		WithValues("Name", name, "Class", class, "Plan", plan, "Namespace", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Config.Org).
		WithStringValue("Class", class).
		WithStringValue("Plan", plan).
		WithTable("Parameter", "Value").
		Msg("Create Service")

	if err := c.TargetOk(); err != nil {
		return err
	}

	request := models.CatalogCreateRequest{
		Name:             name,
		Class:            class,
		Plan:             plan,
		Data:             data,
		WaitForProvision: waitForProvision,
	}

	if waitForProvision {
		c.ui.Note().KeeplineUnder(1).Msg("Provisioning...")
		s := c.ui.Progressf("Provisioning")
		defer s.Stop()
	}

	_, err := c.API.ServiceCreate(request, c.Config.Org)
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Config.Org).
		WithStringValue("Class", class).
		WithStringValue("Plan", plan).
		Msg("Service Saved.")

	if waitForProvision {
		c.ui.Success().Msg("Service Provisioned.")
	} else {
		c.ui.Note().Msg(fmt.Sprintf("Use `epinio service %s` to watch when it is provisioned", name))
	}

	return nil
}

// CreateCustomService creates a service specified by name and key/value dictionary
// TODO: Allow underscores in service names (right now they fail because of kubernetes naming rules for secrets)
func (c *EpinioClient) CreateCustomService(name string, dict []string) error {
	log := c.Log.WithName("Create Custom Service").
		WithValues("Name", name, "Namespace", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

	data := make(map[string]string)
	msg := c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Config.Org).
		WithTable("Parameter", "Value")
	for i := 0; i < len(dict); i += 2 {
		key := dict[i]
		value := dict[i+1]
		msg = msg.WithTableRow(key, value)
		data[key] = value
	}
	msg.Msg("Create Custom Service")

	if err := c.TargetOk(); err != nil {
		return err
	}

	request := models.CustomCreateRequest{
		Name: name,
		Data: data,
	}

	_, err := c.API.ServiceCreateCustom(request, c.Config.Org)
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Config.Org).
		Msg("Service Saved.")
	return nil
}

// ServiceDetails shows the information of a service specified by name
func (c *EpinioClient) ServiceDetails(name string) error {
	log := c.Log.WithName("Service Details").
		WithValues("Name", name, "Namespace", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Namespace", c.Config.Org).
		Msg("Service Details")

	if err := c.TargetOk(); err != nil {
		return err
	}

	resp, err := c.API.ServiceShow(c.Config.Org, name)
	if err != nil {
		return err
	}
	serviceDetails := resp.Details

	msg := c.ui.Success().WithTable("", "")
	keys := make([]string, 0, len(serviceDetails))
	for k := range serviceDetails {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		msg = msg.WithTableRow(k, serviceDetails[k])
	}

	msg.Msg("")
	return nil
}

// Info displays information about environment
func (c *EpinioClient) Info() error {
	log := c.Log.WithName("Info")
	log.Info("start")
	defer log.Info("return")

	v, err := c.API.Info()
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Platform", v.Platform).
		WithStringValue("Kubernetes Version", v.KubeVersion).
		WithStringValue("Epinio Version", v.Version).
		Msg("Epinio Environment")

	return nil
}

// AppsMatching returns all Epinio apps having the specified prefix
// in their name.
func (c *EpinioClient) AppsMatching(ctx context.Context, prefix string) []string {
	log := c.Log.WithName("AppsMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	// Ask for all apps. Filtering is local.
	// TODO: Create new endpoint (compare `EnvMatch`) and move filtering to the server.

	apps, err := c.API.Apps(c.Config.Org)
	if err != nil {
		return result
	}

	for _, app := range apps {
		details.Info("Found", "Name", app.Name)

		if strings.HasPrefix(app.Name, prefix) {
			details.Info("Matched", "Name", app.Name)
			result = append(result, app.Name)
		}
	}

	sort.Strings(result)

	return result
}

// Apps gets all Epinio apps in the targeted org, or all apps in all namespaces
func (c *EpinioClient) Apps(all bool) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	msg := c.ui.Note()
	if all {
		msg.Msg("Listing all applications")
	} else {
		msg.
			WithStringValue("Namespace", c.Config.Org).
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
		apps, err = c.API.Apps(c.Config.Org)
	}
	if err != nil {
		return err
	}

	sort.Sort(apps)

	if all {
		msg = c.ui.Success().WithTable("Namespace", "Name", "Status", "Routes", "Services")

		for _, app := range apps {
			msg = msg.WithTableRow(
				app.Organization,
				app.Name,
				app.Status,
				app.Route,
				strings.Join(app.BoundServices, ", "))
		}
	} else {
		msg = c.ui.Success().WithTable("Name", "Status", "Routes", "Services")

		for _, app := range apps {
			msg = msg.WithTableRow(
				app.Name,
				app.Status,
				app.Route,
				strings.Join(app.BoundServices, ", "))
		}
	}

	msg.Msg("Epinio Applications:")

	return nil
}

// AppShow displays the information of the named app, in the targeted org
func (c *EpinioClient) AppShow(appName string) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Config.Org, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Namespace", c.Config.Org).
		WithStringValue("Application", appName).
		Msg("Show application details")

	if err := c.TargetOk(); err != nil {
		return err
	}

	details.Info("show application")

	app, err := c.API.AppShow(c.Config.Org, appName)
	if err != nil {
		return err
	}

	c.ui.Success().
		WithTable("Key", "Value").
		WithTableRow("Status", app.Status).
		WithTableRow("Username", app.Username).
		WithTableRow("StageId", app.StageID).
		WithTableRow("Routes", app.Route).
		WithTableRow("Services", strings.Join(app.BoundServices, ", ")).
		WithTableRow("Environment", `See it by running the command "epinio app env list `+appName+`"`).
		Msg("Details:")

	return nil
}

// AppStageID returns the stage id of the named app, in the targeted org
func (c *EpinioClient) AppStageID(appName string) (string, error) {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Config.Org, "Application", appName)
	log.Info("start")
	defer log.Info("return")

	app, err := c.API.AppShow(c.Config.Org, appName)
	if err != nil {
		return "", err
	}

	if !app.Active {
		return "", errors.New("Application has no workload")
	}

	return app.StageID, nil
}

// AppUpdate updates the specified running application's attributes (e.g. instances)
func (c *EpinioClient) AppUpdate(appName string, instances int32) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Config.Org, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Namespace", c.Config.Org).
		WithStringValue("Application", appName).
		Msg("Update application")

	if err := c.TargetOk(); err != nil {
		return err
	}

	details.Info("update application")

	req := models.UpdateAppRequest{Instances: instances}
	_, err := c.API.AppUpdate(req, c.Config.Org, appName)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Successfully updated application")

	return nil
}

// AppLogs streams the logs of all the application instances, in the targeted org
// If stageID is an empty string, runtime application logs are streamed. If stageID
// is set, then the matching staging logs are streamed.
// There are 2 ways of stopping this method:
// 1. The websocket connection closes.
// 2. Something is sent to the interrupt channel
// The interrupt channel is used by the caller when printing of logs should
// be stopped.
// To make sure everything is properly stopped (both the main thread and the
// go routine) no matter what caused the stop (number 1 or 2 above):
// - The go routines closes the connection on interrupt. This causes the main
//   loop to stop as well.
// - The main thread sends a signal to the `done` channel when it returns. This
//   causes the go routine to stop.
// - The main thread waits for the go routine to stop before finally returning (by
//   calling `wg.Wait()`.
// This is what happens when `interrupt` receives something:
// 1. The go routine closes the connection
// 2. The loop in the main thread is stopped because the connection was closed
// 3. The main thread sends to the `done` chan (as a "defer" function), and then
//    calls wg.Wait() to wait for the go routine to exit.
// 4. The go routine receives the `done` message, calls wg.Done() and returns
// 5. The main thread returns
// When the connection is closed (e.g. from the server side), the process is the
// same but starts from #2 above.
// TODO move into transport package
func (c *EpinioClient) AppLogs(appName, stageID string, follow bool, interrupt chan bool) error {
	log := c.Log.WithName("Apps").WithValues("Namespace", c.Config.Org, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Namespace", c.Config.Org).
		WithStringValue("Application", appName).
		Msg("Streaming application logs")

	if err := c.TargetOk(); err != nil {
		return err
	}

	details.Info("application logs")

	var urlArgs = []string{}
	urlArgs = append(urlArgs, fmt.Sprintf("follow=%t", follow))
	urlArgs = append(urlArgs, fmt.Sprintf("stage_id=%s", stageID))

	headers := http.Header{
		"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.Config.User, c.Config.Password)))},
	}

	var endpoint string
	if stageID == "" {
		endpoint = api.Routes.Path("AppLogs", c.Config.Org, appName)
	} else {
		endpoint = api.Routes.Path("StagingLogs", c.Config.Org, stageID)
	}
	webSocketConn, resp, err := websocket.DefaultDialer.Dial(
		fmt.Sprintf("%s/%s?%s", c.API.WsURL, endpoint, strings.Join(urlArgs, "&")), headers)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to connect to websockets endpoint. Response was = %+v\nThe error is", resp))
	}

	done := make(chan bool)
	// When we get an interrupt, we close the websocket connection and we
	// we don't want to return an error in this case.
	connectionClosedByUs := false

	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()
	go func() { // Closes the connection on "interrupt" or just stops on "done"
		defer wg.Done()
		for {
			select {
			case <-done: // Used by the other loop stop stop this go routine
				return
			case <-interrupt:
				// Used by the caller of this method to stop everything. We simply close
				// the connection here. This will make the loop below to stop and send us
				// a signal on "done" above. That will stop this go routine too.
				// nolint:errcheck // no place to pass any error to.
				webSocketConn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
				connectionClosedByUs = true
				webSocketConn.Close()
			}
		}
	}()

	defer func() {
		done <- true // Stop the go routine when we return
	}()

	var logLine tailer.ContainerLogLine
	printer := logprinter.LogPrinter{Tmpl: logprinter.DefaultSingleNamespaceTemplate()}
	for {
		_, message, err := webSocketConn.ReadMessage()
		if err != nil {
			if connectionClosedByUs {
				return nil
			}
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				webSocketConn.Close()
				return nil
			}
			return err
		}
		err = json.Unmarshal(message, &logLine)
		if err != nil {
			return err
		}

		printer.Print(logprinter.Log{
			Message:       logLine.Message,
			Namespace:     logLine.Namespace,
			PodName:       logLine.PodName,
			ContainerName: logLine.ContainerName,
		}, c.ui.ProgressNote().Compact())
	}
}

// CreateOrg creates an Org namespace
func (c *EpinioClient) CreateOrg(org string) error {
	log := c.Log.WithName("CreateNamespace").WithValues("Namespace", org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Creating namespace...")

	errorMsgs := validation.IsDNS1123Subdomain(org)
	if len(errorMsgs) > 0 {
		return fmt.Errorf("%s: %s", "org name incorrect", strings.Join(errorMsgs, "\n"))
	}

	_, err := c.API.NamespaceCreate(models.NamespaceCreateRequest{Name: org})
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Namespace created.")

	return nil
}

// DeleteOrg deletes an Org
func (c *EpinioClient) DeleteOrg(org string) error {
	log := c.Log.WithName("DeleteNamespace").WithValues("Namespace", org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Deleting namespace...")

	_, err := c.API.NamespaceDelete(org)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Namespace deleted.")

	return nil
}

// Delete removes the named application from the cluster
func (c *EpinioClient) Delete(ctx context.Context, appname string) error {
	log := c.Log.WithName("Delete").WithValues("Application", appname)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", appname).
		WithStringValue("Namespace", c.Config.Org).
		Msg("Deleting application...")

	if err := c.TargetOk(); err != nil {
		return err
	}

	s := c.ui.Progressf("Deleting %s in %s", appname, c.Config.Org)
	defer s.Stop()

	response, err := c.API.AppDelete(c.Config.Org, appname)
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

// OrgsMatching returns all Epinio orgs having the specified prefix in their name
func (c *EpinioClient) OrgsMatching(prefix string) []string {
	log := c.Log.WithName("NamespaceMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")

	result := []string{}

	resp, err := c.API.NamespacesMatch(prefix)
	if err != nil {
		return result
	}

	result = resp.Names

	log.Info("matches", "found", result)
	return result
}

func (c *EpinioClient) Orgs() error {
	log := c.Log.WithName("Namespaces")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().Msg("Listing namespaces")

	details.Info("list namespaces")

	namespaces, err := c.API.Namespaces()
	if err != nil {
		return err
	}

	sort.Sort(namespaces)
	msg := c.ui.Success().WithTable("Name", "Applications", "Services")

	for _, namespace := range namespaces {
		sort.Strings(namespace.Apps)
		sort.Strings(namespace.Services)
		msg = msg.WithTableRow(
			namespace.Name,
			strings.Join(namespace.Apps, ", "),
			strings.Join(namespace.Services, ", "))
	}

	msg.Msg("Epinio Namespaces:")

	return nil
}

// Target targets an org
func (c *EpinioClient) Target(org string) error {
	log := c.Log.WithName("Target").WithValues("Namespace", org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	if org == "" {
		details.Info("query config")
		c.ui.Success().
			WithStringValue("Currently targeted namespace", c.Config.Org).
			Msg("")
		return nil
	}

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Targeting namespace...")

	// TODO: Validation of the org name removed. Proper validation
	// of the targeted org is done by all the other commands using
	// it anyway. If we really want it here and now, implement an
	// `namespace show` command and API, and then use that API for the
	// validation.

	details.Info("set config")
	c.Config.Org = org
	err := c.Config.Save()
	if err != nil {
		return errors.Wrap(err, "failed to save configuration")
	}

	c.ui.Success().Msg("Namespace targeted.")

	return nil
}

func (c *EpinioClient) TargetOk() error {
	if c.Config.Org == "" {
		return errors.New("Internal Error: No namespace targeted")
	}
	return nil
}

func uniqueStrings(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
