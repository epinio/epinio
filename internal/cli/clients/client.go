package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/go-logr/logr"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/suse/carrier/deployments"
	"github.com/suse/carrier/internal/application"
	"github.com/suse/carrier/internal/cli/config"
	"github.com/suse/carrier/internal/duration"
	carriergitea "github.com/suse/carrier/internal/gitea"
	"github.com/suse/carrier/internal/services"
	"github.com/suse/carrier/kubernetes"
	kubeconfig "github.com/suse/carrier/kubernetes/config"
	"github.com/suse/carrier/kubernetes/tailer"
	"github.com/suse/carrier/termui"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	// HookSecret should be generated
	// TODO: generate this and put it in a secret
	HookSecret = "74tZTBHkhjMT5Klj6Ik6PqmM"

	// StagingEventListenerURL should not exist
	// TODO: detect this based on namespaces and services
	StagingEventListenerURL = "http://el-staging-listener." + deployments.WorkloadsDeploymentID + ":8080"
)

// CarrierClient provides functionality for talking to a
// Carrier installation on Kubernetes
type CarrierClient struct {
	GiteaClient   *gitea.Client
	KubeClient    *kubernetes.Cluster
	Config        *config.Config
	Log           logr.Logger
	ui            *termui.UI
	giteaResolver *carriergitea.Resolver
	serverURL     string
}

func NewCarrierClient(flags *pflag.FlagSet) (*CarrierClient, error) {
	configConfig, err := config.Load(flags)
	if err != nil {
		return nil, err
	}
	restConfig, err := kubeconfig.KubeConfig()
	if err != nil {
		return nil, err
	}
	cluster, err := kubernetes.NewClusterFromClient(restConfig)
	if err != nil {
		return nil, err
	}
	resolver := carriergitea.NewResolver(configConfig, cluster)
	client, err := carriergitea.NewGiteaClient(resolver)
	if err != nil {
		return nil, err
	}
	uiUI := termui.NewUI()
	// TODO: This is a hack. We won't need it when the server will be running
	// in-cluster. Then server-url will never empty.
	serverURL := ""
	if flags != nil {
		if urlFromFlags, err := flags.GetString("server-url"); err == nil {
			serverURL = urlFromFlags
		}
	}

	logger := kubeconfig.NewClientLogger()
	carrierClient := &CarrierClient{
		GiteaClient:   client,
		KubeClient:    cluster,
		ui:            uiUI,
		Config:        configConfig,
		giteaResolver: resolver,
		Log:           logger,
		serverURL:     serverURL,
	}
	return carrierClient, nil
}

// ServicePlans gets all service classes in the cluster, for the
// specified class
func (c *CarrierClient) ServicePlans(serviceClassName string) error {
	log := c.Log.WithName("ServicePlans").WithValues("ServiceClass", serviceClassName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		Msg("Listing service plans")

	serviceClass, err := services.ClassLookup(c.KubeClient, serviceClassName)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	if serviceClass == nil {
		c.ui.Exclamation().Msg("Service Class does not exist")
		return nil
	}

	servicePlans, err := serviceClass.ListPlans()
	if err != nil {
		return errors.Wrap(err, "failed to list service plans")
	}

	// todo: sort service plans by name before display

	details.Info("list service plans")

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
	msg.Msg("Carrier Service Plans:")

	return nil
}

// ServicePlanMatching gets all service plans in the cluster, for the
// specified class, and the given prefix
func (c *CarrierClient) ServicePlanMatching(serviceClassName, prefix string) []string {
	log := c.Log.WithName("ServicePlans").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	serviceClass, err := services.ClassLookup(c.KubeClient, serviceClassName)
	if err != nil {
		return result
	}

	servicePlans, err := serviceClass.ListPlans()
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

// ServiceClasses gets all service classes in the cluster
func (c *CarrierClient) ServiceClasses() error {
	log := c.Log.WithName("ServiceClasses")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		Msg("Listing service classes")

	serviceClasses, err := services.ListClasses(c.KubeClient)
	if err != nil {
		return errors.Wrap(err, "failed to list service classes")
	}

	// todo: sort service classes by name before display

	details.Info("list service classes")

	msg := c.ui.Success().WithTable("Name", "Description", "Broker")
	for _, sc := range serviceClasses {
		msg = msg.WithTableRow(sc.Name, sc.Description, sc.Broker)
	}
	msg.Msg("Carrier Service Classes:")

	return nil
}

// ServiceClassMatching returns all service classes in the cluster which have the specified prefix in their name
func (c *CarrierClient) ServiceClassMatching(prefix string) []string {
	log := c.Log.WithName("ServiceClasses").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	serviceClasses, err := services.ListClasses(c.KubeClient)
	if err != nil {
		details.Info("Error", err)
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

// Services gets all Carrier services in the targeted org
func (c *CarrierClient) Services() error {
	log := c.Log.WithName("Services").WithValues("Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Organization", c.Config.Org).
		Msg("Listing services")

	// todo: fix, remove, move to server
	details.Info("validate")
	err := c.ensureGoodOrg(c.Config.Org, "Unable to list services.")
	if err != nil {
		return err
	}

	orgServices, err := services.List(c.KubeClient, c.Config.Org)
	if err != nil {
		return errors.Wrap(err, "failed to list services")
	}

	appsOf, err := c.servicesToApps(c.Config.Org)
	if err != nil {
		return errors.Wrap(err, "failed to list apps per service")
	}

	// todo: sort services by name before display

	details.Info("list services")

	msg := c.ui.Success().WithTable("Name", "Applications")
	for _, s := range orgServices {
		var bound string
		if theapps, found := appsOf[s.Name()]; found {
			appnames := []string{}
			for _, app := range theapps {
				appnames = append(appnames, app.Name)
			}
			bound = strings.Join(appnames, ", ")
		} else {
			bound = ""
		}
		msg = msg.WithTableRow(s.Name(), bound)
	}
	msg.Msg("Carrier Services:")

	return nil
}

// ServiceMatching returns all Carrier services having the specified prefix
// in their name.
func (c *CarrierClient) ServiceMatching(prefix string) []string {
	log := c.Log.WithName("ServiceMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	orgServices, err := services.List(c.KubeClient, c.Config.Org)
	if err != nil {
		return result
	}

	for _, s := range orgServices {
		service := s.Name()
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
func (c *CarrierClient) BindService(serviceName, appName string) error {
	log := c.Log.WithName("Bind Service To Application").
		WithValues("Name", serviceName, "Application", appName, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Organization", c.Config.Org).
		Msg("Bind Service")

	// todo: fix, remove, move to server
	details.Info("validate")
	err := c.ensureGoodOrg(c.Config.Org, "Unable to bind service.")
	if err != nil {
		return err
	}

	// Lookup app and service. Conversion from name to internal objects.

	app, err := application.Lookup(c.KubeClient, c.GiteaClient, c.Config.Org, appName)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	service, err := services.Lookup(c.KubeClient, c.Config.Org, serviceName)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	err = app.Bind(service)
	if err != nil {
		return errors.Wrap(err, "failed to bind service")
	}

	c.ui.Success().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Organization", c.Config.Org).
		Msg("Service Bound to Application.")
	return nil
}

// UnbindService detaches the service specified by name from the named
// application, both in the targeted organization.
func (c *CarrierClient) UnbindService(serviceName, appName string) error {
	log := c.Log.WithName("Unbind Service").
		WithValues("Name", serviceName, "Application", appName, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Organization", c.Config.Org).
		Msg("Unbind Service from Application")

	// todo: fix, remove, move to server
	details.Info("validate")
	err := c.ensureGoodOrg(c.Config.Org, "Unable to unbind service.")
	if err != nil {
		return err
	}

	// Lookup app and service. Conversion from names to internal objects.

	app, err := application.Lookup(c.KubeClient, c.GiteaClient, c.Config.Org, appName)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	service, err := services.Lookup(c.KubeClient, c.Config.Org, serviceName)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	// Do the task

	err = app.Unbind(service)

	if err != nil {
		return errors.Wrap(err, "failed to unbind service")
	}

	c.ui.Success().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Organization", c.Config.Org).
		Msg("Service Detached From Application.")
	return nil
}

// DeleteService deletes a service specified by name
func (c *CarrierClient) DeleteService(name string, unbind bool) error {
	log := c.Log.WithName("Delete Service").
		WithValues("Name", name, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.Config.Org).
		Msg("Delete Service")

	// todo: fix, remove, move to server
	details.Info("validate")
	err := c.ensureGoodOrg(c.Config.Org, "Unable to remove service.")
	if err != nil {
		return err
	}

	service, err := services.Lookup(c.KubeClient, c.Config.Org, name)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	appsOf, err := c.servicesToApps(c.Config.Org)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	if boundApps, found := appsOf[service.Name()]; found {
		var msg *termui.Message
		if !unbind {
			msg = c.ui.Exclamation()
		} else {
			msg = c.ui.Note()
		}

		msg = msg.WithTable("Bound Applications")
		for _, app := range boundApps {
			msg = msg.WithTableRow(app.Name)
		}

		if !unbind {
			msg.Msg("Unable to delete service. It is still used by")
			c.ui.Exclamation().Compact().Msg("Use --unbind to force the issue")
			return nil
		} else {
			msg.Msg("Unbinding Service From Using Applications Before Deletion")
			c.ui.Exclamation().
				Timeout(5 * time.Second).
				Msg("Hit Enter to continue or Ctrl+C to abort (removal will continue automatically in 5 seconds)")

			for _, app := range boundApps {
				c.ui.Note().WithStringValue("Application", app.Name).Msg("Unbinding")
				err = app.Unbind(service)
				if err != nil {
					c.ui.Exclamation().Msg(err.Error())
					continue
				}
				c.ui.Success().Compact().Msg("Unbound")
			}

			c.ui.Note().Msg("Back to deleting the service...")
		}
	}

	err = service.Delete()
	if err != nil {
		return errors.Wrap(err, "failed to delete service")
	}

	c.ui.Success().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.Config.Org).
		Msg("Service Removed.")
	return nil
}

// CreateService creates a service specified by name, class, plan, and optional key/value dictionary
// TODO: Allow underscores in service names (right now they fail because of kubernetes naming rules for secrets)
func (c *CarrierClient) CreateService(name, class, plan string, dict []string, waitForProvision bool) error {
	log := c.Log.WithName("Create Service").
		WithValues("Name", name, "Class", class, "Plan", plan, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	data := make(map[string]string)
	msg := c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.Config.Org).
		WithStringValue("Class", class).
		WithStringValue("Plan", plan).
		WithTable("Parameter", "Value")
	for i := 0; i < len(dict); i += 2 {
		key := dict[i]
		value := dict[i+1]
		msg = msg.WithTableRow(key, value)
		data[key] = value
	}
	msg.Msg("Create Service")

	// todo: fix, remove, move to server
	details.Info("validate")
	err := c.ensureGoodOrg(c.Config.Org, "Unable to create service.")
	if err != nil {
		return err
	}

	service, err := services.CreateCatalogService(c.KubeClient, name, c.Config.Org, class, plan, data)
	if err != nil {
		return errors.Wrap(err, "failed to create secret")
	}

	c.ui.Success().
		WithStringValue("Name", service.Name()).
		WithStringValue("Organization", service.Org()).
		WithStringValue("Class", class).
		WithStringValue("Plan", plan).
		Msg("Service Saved.")

	if waitForProvision {
		c.ui.Note().KeeplineUnder(1).Msg("Provisioning...")
		s := c.ui.Progressf("Provisioning")
		defer s.Stop()

		err := service.WaitForProvision()
		if err != nil {
			return errors.Wrap(err, "failed waiting for the service to be provisioned")
		}

		c.ui.Success().Msg("Service Provisioned.")
	} else {
		c.ui.Note().Msg(fmt.Sprintf("Use `carrier service %s` to watch when it is provisioned", service.Name()))
	}

	return nil
}

// CreateCustomService creates a service specified by name and key/value dictionary
// TODO: Allow underscores in service names (right now they fail because of kubernetes naming rules for secrets)
func (c *CarrierClient) CreateCustomService(name string, dict []string) error {
	log := c.Log.WithName("Create Custom Service").
		WithValues("Name", name, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	data := make(map[string]string)
	msg := c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.Config.Org).
		WithTable("Parameter", "Value")
	for i := 0; i < len(dict); i += 2 {
		key := dict[i]
		value := dict[i+1]
		msg = msg.WithTableRow(key, value)
		data[key] = value
	}
	msg.Msg("Create Custom Service")

	// todo: fix, remove, move to server
	details.Info("validate")
	err := c.ensureGoodOrg(c.Config.Org, "Unable to create service.")
	if err != nil {
		return err
	}

	service, err := services.CreateCustomService(c.KubeClient, name, c.Config.Org, data)
	if err != nil {
		return errors.Wrap(err, "failed to create secret")
	}

	c.ui.Success().
		WithStringValue("Name", service.Name()).
		WithStringValue("Organization", service.Org()).
		Msg("Service Saved.")
	return nil
}

// ServiceDetails shows the information of a service specified by name
func (c *CarrierClient) ServiceDetails(name string) error {
	log := c.Log.WithName("Service Details").
		WithValues("Name", name, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.Config.Org).
		Msg("Service Details")

	// todo: fix, remove, move to server
	details.Info("validate")
	err := c.ensureGoodOrg(c.Config.Org, "Unable to detail service.")
	if err != nil {
		return err
	}

	service, err := services.Lookup(c.KubeClient, c.Config.Org, name)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	status, err := service.Status()
	if err != nil {
		return errors.Wrap(err, "failed to detail service")
	}

	serviceDetails, err := service.Details()
	if err != nil {
		return errors.Wrap(err, "failed to detail service")
	}

	msg := c.ui.Success().WithTable("", "")
	msg = msg.WithTableRow("Status", status)

	// Show the service details in sorted order.
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
func (c *CarrierClient) Info() error {
	log := c.Log.WithName("Info")
	log.Info("start")
	defer log.Info("return")

	platform := c.KubeClient.GetPlatform()
	kubeVersion, err := c.KubeClient.GetVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get kube version")
	}

	giteaVersion := "unavailable"

	version, resp, err := c.GiteaClient.ServerVersion()
	if err == nil && resp != nil && resp.StatusCode == 200 {
		giteaVersion = version
	}

	c.ui.Success().
		WithStringValue("Platform", platform.String()).
		WithStringValue("Kubernetes Version", kubeVersion).
		WithStringValue("Gitea Version", giteaVersion).
		Msg("Carrier Environment")

	return nil
}

// AppsMatching returns all Carrier apps having the specified prefix
// in their name.
func (c *CarrierClient) AppsMatching(prefix string) []string {
	log := c.Log.WithName("AppsMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	apps, err := application.List(c.KubeClient, c.GiteaClient, c.Config.Org)
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

	return result
}

// Apps gets all Carrier apps in the targeted org
func (c *CarrierClient) Apps() error {
	log := c.Log.WithName("Apps").WithValues("Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Organization", c.Config.Org).
		Msg("Listing applications")

	details.Info("validate")
	// todo: fix, remove, move to server
	// TODO: remove this
	err := c.ensureGoodOrg(c.Config.Org, "Unable to list applications.")
	if err != nil {
		return err
	}

	details.Info("list applications")

	jsonResponse, err := c.curl(fmt.Sprintf("api/v1/org/%s/applications", c.Config.Org), "GET", "")
	if err != nil {
		return err
	}
	var apps application.ApplicationList
	if err := json.Unmarshal(jsonResponse, &apps); err != nil {
		return err
	}

	msg := c.ui.Success().WithTable("Name", "Status", "Routes", "Services")

	for _, app := range apps {
		msg = msg.WithTableRow(
			app.Name,
			app.Status,
			strings.Join(app.Routes, ", "),
			strings.Join(app.BoundServices, ", "))
	}

	msg.Msg("Carrier Applications:")

	return nil
}

// AppShow displays the information of the named app, in the targeted org
func (c *CarrierClient) AppShow(appName string) error {
	log := c.Log.WithName("Apps").WithValues("Organization", c.Config.Org, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Organization", c.Config.Org).
		WithStringValue("Application", appName).
		Msg("Show application details")

	// todo: fix, move into server
	details.Info("validate")
	err := c.ensureGoodOrg(c.Config.Org, "Unable to show application details.")
	if err != nil {
		return err
	}

	details.Info("list applications")

	jsonResponse, err := c.curl(fmt.Sprintf("api/v1/org/%s/applications/%s", c.Config.Org, appName), "GET", "")
	if err != nil {
		return err
	}
	var app application.Application
	if err := json.Unmarshal(jsonResponse, &app); err != nil {
		return err
	}

	msg := c.ui.Success().WithTable("Key", "Value")

	msg = msg.WithTableRow("Status", app.Status)
	msg = msg.WithTableRow("Routes", strings.Join(app.Routes, ", "))
	msg = msg.WithTableRow("Services", strings.Join(app.BoundServices, ", "))

	msg.Msg("Details:")

	return nil
}

// CreateOrg creates an Org in gitea
func (c *CarrierClient) CreateOrg(org string) error {
	log := c.Log.WithName("CreateOrg").WithValues("Organization", org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Creating organization...")

	details.Info("validate")
	details.Info("gitea get-org")
	_, resp, err := c.GiteaClient.GetOrg(org)
	if resp == nil && err != nil {
		return errors.Wrap(err, "failed to make get org request")
	}

	if resp.StatusCode == 200 {
		c.ui.Exclamation().Msg("Organization already exists.")
		return nil
	}

	details.Info("gitea create-org")
	_, _, err = c.GiteaClient.CreateOrg(gitea.CreateOrgOption{
		Name: org,
	})

	if err != nil {
		return errors.Wrap(err, "failed to create org")
	}

	c.ui.Success().Msg("Organization created.")

	return nil
}

// Delete removes the named application from the cluster
func (c *CarrierClient) Delete(appname string) error {
	log := c.Log.WithName("Delete").WithValues("Application", appname)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", appname).
		WithStringValue("Organization", c.Config.Org).
		Msg("Deleting application...")

	app, err := application.Lookup(c.KubeClient, c.GiteaClient,
		c.Config.Org, appname)
	if err != nil {
		return errors.Wrap(err, "failed to find application")
	}

	bound, err := app.Services()
	if err != nil {
		return errors.Wrap(err, "failed to find bound services")
	}
	if len(bound) > 0 {
		msg := c.ui.Note().WithTable("Currently Bound")
		for _, bonded := range bound {
			msg = msg.WithTableRow(bonded.Name())
		}
		msg.Msg("Bound Services Found, Unbind Them")
		for _, bonded := range bound {
			c.ui.Note().WithStringValue("Service", bonded.Name()).Msg("Unbinding")
			err = app.Unbind(bonded)
			if err != nil {
				c.ui.Exclamation().Msg(err.Error())
				continue
			}
			c.ui.Success().Compact().Msg("Unbound")
		}

		c.ui.Note().Msg("Back to deleting the application...")
	}

	c.ui.ProgressNote().KeepLine().Msg("Deleting...")

	err = app.Delete()

	// The command above removes the application's deployment.
	// This in turn deletes the associated replicaset, and pod, in
	// this order. The pod being gone thus indicates command
	// completion, and is therefore what we are waiting on below.

	err = c.KubeClient.WaitForPodBySelectorMissing(c.ui,
		deployments.WorkloadsDeploymentID,
		fmt.Sprintf("cloudfoundry.org/guid=%s.%s", c.Config.Org, appname),
		duration.ToDeployment())
	if err != nil {
		return errors.Wrap(err, "failed to delete application pod")
	}

	c.ui.Success().Msg("Application deleted.")

	return nil
}

// OrgsMatching returns all Carrier orgs having the specified prefix
// in their name
func (c *CarrierClient) OrgsMatching(prefix string) []string {
	log := c.Log.WithName("OrgsMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	orgs, _, err := c.GiteaClient.AdminListOrgs(gitea.AdminListOrgsOptions{})
	if err != nil {
		return result
	}

	for _, org := range orgs {
		details.Info("Found", "Name", org.UserName)

		if strings.HasPrefix(org.UserName, prefix) {
			details.Info("Matched", "Name", org.UserName)
			result = append(result, org.UserName)
		}
	}

	return result
}

// Orgs get a list of all orgs in gitea
func (c *CarrierClient) Orgs() error {
	log := c.Log.WithName("Orgs")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().Msg("Listing organizations")

	details.Info("gitea admin list orgs")
	orgs, _, err := c.GiteaClient.AdminListOrgs(gitea.AdminListOrgsOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list orgs")
	}

	msg := c.ui.Success().WithTable("Name")

	for _, org := range orgs {
		msg = msg.WithTableRow(org.UserName)
	}

	msg.Msg("Carrier Organizations:")

	return nil
}

// Push pushes an app
func (c *CarrierClient) Push(app string, path string) error {
	log := c.Log.
		WithName("Push").
		WithValues("Name", app,
			"Organization", c.Config.Org,
			"Sources", path)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Name", app).
		WithStringValue("Sources", path).
		WithStringValue("Organization", c.Config.Org).
		Msg("About to push an application with given name and sources into the specified organization")

	c.ui.Exclamation().
		Timeout(duration.UserAbort()).
		Msg("Hit Enter to continue or Ctrl+C to abort (deployment will continue automatically in 5 seconds)")

	// todo: fix, remove, move to server
	details.Info("validate")
	err := c.ensureGoodOrg(c.Config.Org, "Unable to push.")
	if err != nil {
		return err
	}

	details.Info("create repo")
	err = c.createRepo(app)
	if err != nil {
		return errors.Wrap(err, "create repo failed")
	}

	details.Info("create repo webhook")
	err = c.createRepoWebhook(app)
	if err != nil {
		return errors.Wrap(err, "webhook configuration failed")
	}

	details.Info("prepare code")
	tmpDir, err := c.prepareCode(app, c.Config.Org, path)
	if err != nil {
		return errors.Wrap(err, "failed to prepare code")
	}
	defer os.RemoveAll(tmpDir)

	details.Info("git push")
	err = c.gitPush(app, tmpDir)
	if err != nil {
		return errors.Wrap(err, "failed to git push code")
	}

	details.Info("start tailing logs")
	stopFunc, err := c.logs(app)
	if err != nil {
		return errors.Wrap(err, "failed to tail logs")
	}
	defer stopFunc()

	details.Info("wait for app")
	err = c.waitForApp(c.Config.Org, app)
	if err != nil {
		return errors.Wrap(err, "waiting for app failed")
	}

	details.Info("get app default route")
	route, err := c.appDefaultRoute(app)
	if err != nil {
		return errors.Wrap(err, "failed to determine default app route")
	}

	c.ui.Success().
		WithStringValue("Name", app).
		WithStringValue("Organization", c.Config.Org).
		WithStringValue("Route", fmt.Sprintf("https://%s", route)).
		Msg("App is online.")

	return nil
}

// Target targets an org in gitea
func (c *CarrierClient) Target(org string) error {
	log := c.Log.WithName("Target").WithValues("Organization", org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	if org == "" {
		details.Info("query config")
		c.ui.Success().
			WithStringValue("Currently targeted organization", c.Config.Org).
			Msg("")
		return nil
	}

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Targeting organization...")

	// todo: fix, remove, move to server
	details.Info("validate")
	err := c.ensureGoodOrg(org, "Unable to target.")
	if err != nil {
		return err
	}

	details.Info("set config")
	c.Config.Org = org
	err = c.Config.Save()
	if err != nil {
		return errors.Wrap(err, "failed to save configuration")
	}

	c.ui.Success().Msg("Organization targeted.")

	return nil
}

func (c *CarrierClient) check() {
	c.GiteaClient.GetMyUserInfo()
}

func (c *CarrierClient) createRepo(name string) error {
	_, resp, err := c.GiteaClient.GetRepo(c.Config.Org, name)
	if resp == nil && err != nil {
		return errors.Wrap(err, "failed to make get repo request")
	}

	if resp.StatusCode == 200 {
		c.ui.Note().Msg("Application already exists. Updating.")
		return nil
	}

	_, _, err = c.GiteaClient.CreateOrgRepo(c.Config.Org, gitea.CreateRepoOption{
		Name:          name,
		AutoInit:      true,
		Private:       true,
		DefaultBranch: "main",
	})

	if err != nil {
		return errors.Wrap(err, "failed to create application")
	}

	c.ui.Success().Msg("Application Repository created.")

	return nil
}

func (c *CarrierClient) createRepoWebhook(name string) error {
	hooks, _, err := c.GiteaClient.ListRepoHooks(c.Config.Org, name, gitea.ListHooksOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list webhooks")
	}

	for _, hook := range hooks {
		url := hook.Config["url"]
		if url == StagingEventListenerURL {
			c.ui.Normal().Msg("Webhook already exists.")
			return nil
		}
	}

	c.ui.Normal().Msg("Creating webhook in the repo...")

	c.GiteaClient.CreateRepoHook(c.Config.Org, name, gitea.CreateHookOption{
		Active:       true,
		BranchFilter: "*",
		Config: map[string]string{
			"secret":       HookSecret,
			"http_method":  "POST",
			"url":          StagingEventListenerURL,
			"content_type": "json",
		},
		Type: "gitea",
	})

	return nil
}

func (c *CarrierClient) appDefaultRoute(name string) (string, error) {
	domain, err := c.giteaResolver.GetMainDomain()
	if err != nil {
		return "", errors.Wrap(err, "failed to determine carrier domain")
	}

	return fmt.Sprintf("%s.%s", name, domain), nil
}

func (c *CarrierClient) prepareCode(name, org, appDir string) (tmpDir string, err error) {
	c.ui.Normal().Msg("Preparing code ...")

	tmpDir, err = ioutil.TempDir("", "carrier-app")
	if err != nil {
		return "", errors.Wrap(err, "can't create temp directory")
	}

	err = copy.Copy(appDir, tmpDir)
	if err != nil {
		return "", errors.Wrap(err, "failed to copy app sources to temp location")
	}

	err = os.MkdirAll(filepath.Join(tmpDir, ".kube"), 0700)
	if err != nil {
		return "", errors.Wrap(err, "failed to setup kube resources directory in temp app location")
	}

	route, err := c.appDefaultRoute(name)
	if err != nil {
		return "", errors.Wrap(err, "failed to calculate default app route")
	}

	deploymentTmpl, err := template.New("deployment").Parse(`
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "{{ .Org }}.{{ .AppName }}"
  labels:
    app.kubernetes.io/name: "{{ .AppName }}"
    app.kubernetes.io/part-of: "{{ .Org }}"
    app.kubernetes.io/component: application
    app.kubernetes.io/managed-by: carrier
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: "{{ .AppName }}"
  template:
    metadata:
      labels:
        app.kubernetes.io/name: "{{ .AppName }}"
        app.kubernetes.io/part-of: "{{ .Org }}"
        app.kubernetes.io/component: application
        app.kubernetes.io/managed-by: carrier
        # Needed for the ingress extension to work:
        cloudfoundry.org/guid:  "{{ .Org }}.{{ .AppName }}"
      annotations:
        # Needed for the ingress extension to work:
        cloudfoundry.org/routes: '[{ "hostname": "{{ .Route}}", "port": 8080 }]'
        cloudfoundry.org/application_name:  "{{ .AppName }}"
        # Needed for putting kubernetes generic labels on svc and ingress
        eirinix.suse.org/CopyKubeGenericLabels: "true"
    spec:
      serviceAccountName: ` + deployments.WorkloadsDeploymentID + `
      automountServiceAccountToken: false
      containers:
      - name: "{{ .AppName }}"
        image: "127.0.0.1:30500/apps/{{ .AppName }}"
        ports:
        - containerPort: 8080
        env:
        - name: PORT
          value: "8080"
  `)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse deployment template for app")
	}

	appFile, err := os.Create(filepath.Join(tmpDir, ".kube", "app.yml"))
	if err != nil {
		return "", errors.Wrap(err, "failed to create file for kube resource definitions")
	}
	defer func() { err = appFile.Close() }()

	err = deploymentTmpl.Execute(appFile, struct {
		AppName string
		Route   string
		Org     string
	}{
		AppName: name,
		Route:   route,
		Org:     c.Config.Org,
	})
	if err != nil {
		return "", errors.Wrap(err, "failed to render kube resource definition")
	}

	return
}

func (c *CarrierClient) gitPush(name, tmpDir string) error {
	c.ui.Normal().Msg("Pushing application code ...")

	giteaURL, err := c.giteaResolver.GetGiteaURL()
	if err != nil {
		return errors.Wrap(err, "failed to resolve gitea host")
	}

	u, err := url.Parse(giteaURL)
	if err != nil {
		return errors.Wrap(err, "failed to parse gitea url")
	}

	username, password, err := c.giteaResolver.GetGiteaCredentials()
	if err != nil {
		return errors.Wrap(err, "failed to resolve gitea credentials")
	}

	u.User = url.UserPassword(username, password)
	u.Path = path.Join(u.Path, c.Config.Org, name)

	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf(`
cd "%s" 
git init
git config user.name "Carrier"
git config user.email ci@carrier
git remote add carrier "%s"
git fetch --all
git reset --soft carrier/main
git add --all
git commit -m "pushed at %s"
git push carrier %s:main
`, tmpDir, u.String(), time.Now().Format("20060102150405"), "`git branch --show-current`"))

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.ui.Problem().
			WithStringValue("Stdout", string(output)).
			WithStringValue("Stderr", "").
			Msg("App push failed")
		return errors.Wrap(err, "push script failed")
	}

	c.ui.Note().V(1).WithStringValue("Output", string(output)).Msg("")
	c.ui.Success().Msg("Application push successful")

	return nil
}

func (c *CarrierClient) logs(name string) (context.CancelFunc, error) {
	c.ui.ProgressNote().V(1).Msg("Tailing application logs ...")

	ctx, cancelFunc := context.WithCancel(context.Background())

	// TODO: improve the way we look for pods, use selectors
	// and watch staging as well
	err := tailer.Run(c.ui, ctx, &tailer.Config{
		ContainerQuery:        regexp.MustCompile(".*"),
		ExcludeContainerQuery: nil,
		ContainerState:        "running",
		Exclude:               nil,
		Include:               nil,
		Timestamps:            false,
		Since:                 duration.LogHistory(),
		AllNamespaces:         false,
		LabelSelector:         labels.Everything(),
		TailLines:             nil,
		Template:              tailer.DefaultSingleNamespaceTemplate(),

		Namespace: deployments.WorkloadsDeploymentID,
		PodQuery:  regexp.MustCompile(fmt.Sprintf(".*-%s-.*", name)),
	}, c.KubeClient)
	if err != nil {
		return cancelFunc, errors.Wrap(err, "failed to start log tail")
	}

	return cancelFunc, nil
}

func (c *CarrierClient) waitForApp(org, name string) error {
	c.ui.ProgressNote().KeeplineUnder(1).Msg("Creating application resources")
	err := c.KubeClient.WaitUntilPodBySelectorExist(
		c.ui, deployments.WorkloadsDeploymentID,
		fmt.Sprintf("cloudfoundry.org/guid=%s.%s", org, name),
		duration.ToAppBuilt())
	if err != nil {
		return errors.Wrap(err, "waiting for app to be created failed")
	}

	c.ui.ProgressNote().KeeplineUnder(1).Msg("Starting application")

	err = c.KubeClient.WaitForPodBySelectorRunning(
		c.ui, deployments.WorkloadsDeploymentID,
		fmt.Sprintf("cloudfoundry.org/guid=%s.%s", org, name),
		duration.ToPodReady())

	if err != nil {
		return errors.Wrap(err, "waiting for app to come online failed")
	}

	return nil
}

func (c *CarrierClient) ensureGoodOrg(org, msg string) error {
	_, resp, err := c.GiteaClient.GetOrg(org)
	if resp == nil && err != nil {
		return errors.Wrap(err, "failed to make get org request")
	}

	if resp.StatusCode == 404 {
		errmsg := "Organization does not exist."
		if msg != "" {
			errmsg += " " + msg
		}
		c.ui.Exclamation().WithEnd(1).Msg(errmsg)
	}

	return nil
}

func (c *CarrierClient) servicesToApps(org string) (map[string]application.ApplicationList, error) {
	// Determine apps bound to services
	// (inversion of services bound to apps)
	// Literally query apps in the org for their services and invert.

	var appsOf = map[string]application.ApplicationList{}

	apps, err := application.List(c.KubeClient, c.GiteaClient, c.Config.Org)
	if err != nil {
		return nil, err
	}

	for _, app := range apps {
		bound, err := app.Services()
		if err != nil {
			return nil, err
		}
		for _, bonded := range bound {
			bname := bonded.Name()
			if theapps, found := appsOf[bname]; found {
				appsOf[bname] = append(theapps, app)
			} else {
				appsOf[bname] = application.ApplicationList{app}
			}
		}
	}

	return appsOf, nil
}

func (c *CarrierClient) curl(endpoint, method, requestBody string) ([]byte, error) {
	uri := fmt.Sprintf("%s/%s", c.serverURL, endpoint)
	request, err := http.NewRequest(method, uri, strings.NewReader(requestBody))
	if err != nil {
		return []byte{}, err
	}
	response, err := (&http.Client{}).Do(request)
	if err != nil {
		return []byte{}, err
	}
	defer response.Body.Close()

	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return []byte{}, err
	}
	if response.StatusCode != http.StatusOK {
		return bodyBytes, errors.Wrapf(err, "Api responded with %d: ", response.StatusCode)
	}

	return bodyBytes, nil
}
