package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/helpers/tracelog"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/cli/config"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/services"
	"github.com/go-logr/logr"
	archiver "github.com/mholt/archiver/v3"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/dynamic"
)

// EpinioClient provides functionality for talking to a
// Epinio installation on Kubernetes
type EpinioClient struct {
	GiteaClient *gitea.Client
	KubeClient  *kubernetes.Cluster
	Config      *config.Config
	Log         logr.Logger
	ui          *termui.UI
	serverURL   string
}

func NewEpinioClient(flags *pflag.FlagSet) (*EpinioClient, error) {
	configConfig, err := config.Load()
	if err != nil {
		return nil, err
	}

	cluster, err := kubernetes.GetCluster()
	if err != nil {
		return nil, err
	}

	client, err := gitea.New()
	if err != nil {
		return nil, err
	}

	uiUI := termui.NewUI()
	epClient, err := GetEpinioAPIClient()
	if err != nil {
		return nil, err
	}
	serverURL := epClient.URL

	logger := tracelog.NewClientLogger()
	epinioClient := &EpinioClient{
		GiteaClient: client,
		KubeClient:  cluster,
		ui:          uiUI,
		Config:      configConfig,
		Log:         logger,
		serverURL:   serverURL,
	}
	return epinioClient, nil
}

// ServicePlans gets all service classes in the cluster, for the
// specified class
func (c *EpinioClient) ServicePlans(serviceClassName string) error {
	log := c.Log.WithName("ServicePlans").WithValues("ServiceClass", serviceClassName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		Msg("Listing service plans")

	jsonResponse, err := c.get(api.Routes.Path("ServicePlans", serviceClassName))
	if err != nil {
		return err
	}
	var servicePlans services.ServicePlanList
	if err := json.Unmarshal(jsonResponse, &servicePlans); err != nil {
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
func (c *EpinioClient) ServicePlanMatching(serviceClassName, prefix string) []string {
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

// ServiceClassMatching returns all service classes in the cluster which have the specified prefix in their name
func (c *EpinioClient) ServiceClassMatching(prefix string) []string {
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

// ServiceClasses gets all service classes in the cluster
func (c *EpinioClient) ServiceClasses() error {
	log := c.Log.WithName("ServiceClasses")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		Msg("Listing service classes")

	jsonResponse, err := c.get(api.Routes.Path("ServiceClasses"))
	if err != nil {
		return err
	}
	var serviceClasses services.ServiceClassList
	if err := json.Unmarshal(jsonResponse, &serviceClasses); err != nil {
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
	log := c.Log.WithName("Services").WithValues("Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Organization", c.Config.Org).
		Msg("Listing services")

	details.Info("list applications")

	jsonResponse, err := c.get(api.Routes.Path("Services", c.Config.Org))
	if err != nil {
		return err
	}
	var response models.ServiceResponseList
	if err := json.Unmarshal(jsonResponse, &response); err != nil {
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
func (c *EpinioClient) ServiceMatching(prefix string) []string {
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
func (c *EpinioClient) BindService(serviceName, appName string) error {
	log := c.Log.WithName("Bind Service To Application").
		WithValues("Name", serviceName, "Application", appName, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Organization", c.Config.Org).
		Msg("Bind Service")

	request := models.BindRequest{
		Names: []string{serviceName},
	}

	js, err := json.Marshal(request)
	if err != nil {
		return err
	}

	_, err = c.post(api.Routes.Path("ServiceBindingCreate", c.Config.Org, appName), string(js))
	if err != nil {
		return err
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
func (c *EpinioClient) UnbindService(serviceName, appName string) error {
	log := c.Log.WithName("Unbind Service").
		WithValues("Name", serviceName, "Application", appName, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Organization", c.Config.Org).
		Msg("Unbind Service from Application")

	_, err := c.delete(api.Routes.Path("ServiceBindingDelete",
		c.Config.Org, appName, serviceName))
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Organization", c.Config.Org).
		Msg("Service Detached From Application.")
	return nil
}

// DeleteService deletes a service specified by name
func (c *EpinioClient) DeleteService(name string, unbind bool) error {
	log := c.Log.WithName("Delete Service").
		WithValues("Name", name, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.Config.Org).
		Msg("Delete Service")

	request := models.DeleteRequest{
		Unbind: unbind,
	}

	js, err := json.Marshal(request)
	if err != nil {
		return err
	}

	jsonResponse, err := c.curlWithCustomErrorHandling(
		api.Routes.Path("ServiceDelete", c.Config.Org, name),
		"DELETE", string(js),
		func(response *http.Response, bodyBytes []byte, err error) error {
			// nothing special for internal errors and the like
			if response.StatusCode != http.StatusBadRequest {
				return err
			}

			// A bad request happens when the service is
			// still bound to one or more applications,
			// and the response contains an array of their
			// names.

			var apiError map[string][]api.APIError
			if err := json.Unmarshal(bodyBytes, &apiError); err != nil {
				return err
			}

			bound := strings.Split(apiError["errors"][0].Details, ",")

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
		if err.Error() != "Bad Request" {
			return err
		}
		return nil
	}

	if len(jsonResponse) > 0 {
		var deleteResponse models.DeleteResponse
		if err := json.Unmarshal(jsonResponse, &deleteResponse); err != nil {
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
	}

	c.ui.Success().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.Config.Org).
		Msg("Service Removed.")
	return nil
}

// CreateService creates a service specified by name, class, plan, and optional key/value dictionary
// TODO: Allow underscores in service names (right now they fail because of kubernetes naming rules for secrets)
func (c *EpinioClient) CreateService(name, class, plan string, dict []string, waitForProvision bool) error {
	log := c.Log.WithName("Create Service").
		WithValues("Name", name, "Class", class, "Plan", plan, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

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

	request := models.CatalogCreateRequest{
		Name:             name,
		Class:            class,
		Plan:             plan,
		Data:             data,
		WaitForProvision: waitForProvision,
	}

	js, err := json.Marshal(request)
	if err != nil {
		return err
	}

	if waitForProvision {
		c.ui.Note().KeeplineUnder(1).Msg("Provisioning...")
		s := c.ui.Progressf("Provisioning")
		defer s.Stop()
	}

	_, err = c.post(api.Routes.Path("ServiceCreate", c.Config.Org), string(js))
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.Config.Org).
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
		WithValues("Name", name, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

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

	request := models.CustomCreateRequest{
		Name: name,
		Data: data,
	}

	js, err := json.Marshal(request)
	if err != nil {
		return err
	}

	_, err = c.post(api.Routes.Path("ServiceCreateCustom", c.Config.Org),
		string(js))
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.Config.Org).
		Msg("Service Saved.")
	return nil
}

// ServiceDetails shows the information of a service specified by name
func (c *EpinioClient) ServiceDetails(name string) error {
	log := c.Log.WithName("Service Details").
		WithValues("Name", name, "Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.Config.Org).
		Msg("Service Details")

	jsonResponse, err := c.get(api.Routes.Path("ServiceShow", c.Config.Org, name))
	if err != nil {
		return err
	}
	var serviceDetails map[string]string
	if err := json.Unmarshal(jsonResponse, &serviceDetails); err != nil {
		return err
	}

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

	platform := c.KubeClient.GetPlatform()
	kubeVersion, err := c.KubeClient.GetVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get kube version")
	}

	giteaVersion := "unavailable"

	version, resp, err := c.GiteaClient.Client.ServerVersion()
	if err == nil && resp != nil && resp.StatusCode == 200 {
		giteaVersion = version
	}

	epinioVersion := "unavailable"
	if jsonResponse, err := c.get(api.Routes.Path("Info")); err == nil {
		v := struct{ Version string }{}
		if err := json.Unmarshal(jsonResponse, &v); err == nil {
			epinioVersion = v.Version
		}
	}

	c.ui.Success().
		WithStringValue("Platform", platform.String()).
		WithStringValue("Kubernetes Version", kubeVersion).
		WithStringValue("Gitea Version", giteaVersion).
		WithStringValue("Epinio Version", epinioVersion).
		Msg("Epinio Environment")

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

	apps, err := application.List(c.KubeClient, c.Config.Org)
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

// Apps gets all Epinio apps in the targeted org
func (c *EpinioClient) Apps() error {
	log := c.Log.WithName("Apps").WithValues("Organization", c.Config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Organization", c.Config.Org).
		Msg("Listing applications")

	details.Info("list applications")

	jsonResponse, err := c.get(api.Routes.Path("Apps", c.Config.Org))
	if err != nil {
		return err
	}
	var apps application.ApplicationList
	if err := json.Unmarshal(jsonResponse, &apps); err != nil {
		return err
	}

	sort.Sort(apps)
	msg := c.ui.Success().WithTable("Name", "Status", "Routes", "Services")

	for _, app := range apps {
		msg = msg.WithTableRow(
			app.Name,
			app.Status,
			strings.Join(app.Routes, ", "),
			strings.Join(app.BoundServices, ", "))
	}

	msg.Msg("Epinio Applications:")

	return nil
}

// AppShow displays the information of the named app, in the targeted org
func (c *EpinioClient) AppShow(appName string) error {
	log := c.Log.WithName("Apps").WithValues("Organization", c.Config.Org, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Organization", c.Config.Org).
		WithStringValue("Application", appName).
		Msg("Show application details")

	details.Info("list applications")

	jsonResponse, err := c.get(api.Routes.Path("AppShow", c.Config.Org, appName))
	if err != nil {
		return err
	}
	var app application.Application
	if err := json.Unmarshal(jsonResponse, &app); err != nil {
		return err
	}

	c.ui.Success().
		WithTable("Key", "Value").
		WithTableRow("Status", app.Status).
		WithTableRow("Routes", strings.Join(app.Routes, ", ")).
		WithTableRow("Services", strings.Join(app.BoundServices, ", ")).
		Msg("Details:")

	return nil
}

// CreateOrg creates an Org in gitea
func (c *EpinioClient) CreateOrg(org string) error {
	log := c.Log.WithName("CreateOrg").WithValues("Organization", org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Creating organization...")

	errorMsgs := validation.IsDNS1123Subdomain(org)
	if len(errorMsgs) > 0 {
		return fmt.Errorf("%s: %s", "org name incorrect", strings.Join(errorMsgs, "\n"))
	}

	_, err := c.post(api.Routes.Path("Orgs"), fmt.Sprintf(`{ "name": "%s" }`, org))
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Organization created.")

	return nil
}

// DeleteOrg deletes an Org in gitea
func (c *EpinioClient) DeleteOrg(org string) error {
	log := c.Log.WithName("DeleteOrg").WithValues("Organization", org)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Deleting organization...")

	_, err := c.delete(api.Routes.Path("OrgDelete", org))
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Organization deleted.")

	return nil
}

// Delete removes the named application from the cluster
func (c *EpinioClient) Delete(appname string) error {
	log := c.Log.WithName("Delete").WithValues("Application", appname)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", appname).
		WithStringValue("Organization", c.Config.Org).
		Msg("Deleting application...")

	s := c.ui.Progressf("Deleting %s in %s", appname, c.Config.Org)
	defer s.Stop()

	jsonResponse, err := c.delete(api.Routes.Path("AppDelete", c.Config.Org, appname))
	if err != nil {
		return err
	}
	var response map[string][]string
	if err := json.Unmarshal(jsonResponse, &response); err != nil {
		return err
	}

	if !strings.Contains(c.GiteaClient.Domain, "omg.howdoi.website") {
		err = c.deleteProductionCertificate(appname)
		if err != nil {
			return errors.Wrap(err, "failed to delete production certificate")
		}
	} else {
		err = c.deleteLocalCertificate(appname)
		if err != nil {
			return errors.Wrap(err, "failed to delete local certificate")
		}
	}

	unboundServices, ok := response["UnboundServices"]
	if !ok {
		return errors.Errorf("bad response, expected key missing: %v", response)
	}
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
	log := c.Log.WithName("OrgsMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	jsonResponse, err := c.get(api.Routes.Path("Orgs"))
	if err != nil {
		return result
	}

	var orgs []string
	if err := json.Unmarshal(jsonResponse, &orgs); err != nil {
		return result
	}

	for _, org := range orgs {
		details.Info("Found", "Name", org)

		if strings.HasPrefix(org, prefix) {
			details.Info("Matched", "Name", org)
			result = append(result, org)
		}
	}

	return result
}

func (c *EpinioClient) Orgs() error {
	log := c.Log.WithName("Orgs")
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().Msg("Listing organizations")

	details.Info("list organizations")
	jsonResponse, err := c.get(api.Routes.Path("Orgs"))
	if err != nil {
		return err
	}

	var orgs []string
	if err := json.Unmarshal(jsonResponse, &orgs); err != nil {
		return err
	}

	sort.Strings(orgs)
	msg := c.ui.Success().WithTable("Name")

	for _, org := range orgs {
		msg = msg.WithTableRow(org)
	}

	msg.Msg("Epinio Organizations:")

	return nil
}

// Push pushes an app
func (c *EpinioClient) Push(app, source string, services []string) error {
	log := c.Log.
		WithName("Push").
		WithValues("Name", app,
			"Organization", c.Config.Org,
			"Sources", source)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	msg := c.ui.Note().
		WithStringValue("Name", app).
		WithStringValue("Sources", source).
		WithStringValue("Organization", c.Config.Org)

	services = uniqueStrings(services)

	if len(services) > 0 {
		sort.Strings(services)
		msg = msg.WithStringValue("Services:", strings.Join(services, ", "))
	}

	msg.Msg("About to push an application with given name and sources into the specified organization")

	c.ui.Exclamation().
		Timeout(duration.UserAbort()).
		Msg("Hit Enter to continue or Ctrl+C to abort (deployment will continue automatically in 5 seconds)")

	// todo: fix, remove, move to server
	details.Info("validate")
	err := c.ensureGoodOrg(c.Config.Org, "Unable to push.")
	if err != nil {
		return errors.Wrap(err, "ensuring org is good")
	}

	details.Info("validate app name")
	errorMsgs := validation.IsDNS1123Subdomain(app)
	if len(errorMsgs) > 0 {
		return fmt.Errorf("%s: %s", "app name incorrect", strings.Join(errorMsgs, "\n"))
	}

	c.ui.Normal().Msg("Collecting the application sources ...")
	files, err := ioutil.ReadDir(source)
	if err != nil {
		return errors.Wrap(err, "canot read the apps source files")
	}
	sources := []string{}
	for _, f := range files {
		// The FileInfo entries returned by ReadDir provide
		// only the base name of the file or directory they
		// are for. We have to add back the path of the
		// application directory to get the proper paths to
		// the files and directories to assemble in the
		// tarball.

		sources = append(sources, path.Join(source, f.Name()))
	}
	log.V(3).Info("found app data files", "files", sources)

	// create a tmpDir - tarball dir and POST
	tmpDir, err := ioutil.TempDir("", "epinio-app")
	if err != nil {
		return errors.Wrap(err, "can't create temp directory")
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tarball := path.Join(tmpDir, "blob.tar")
	err = archiver.Archive(sources, tarball)
	if err != nil {
		return errors.Wrap(err, "can't create archive")
	}

	c.ui.Normal().Msg("Uploading application code ...")

	// upload blob to the server's uplad API endpoint
	details.Info("upload code")
	uploadResponse, err := c.upload(api.Routes.Path("AppUpload", c.Config.Org, app), tarball)
	if err != nil {
		return errors.Wrap(err, "can't upload archive")
	}

	// returns git commit and app route
	resp := &api.AppResponse{}
	if err := json.Unmarshal(uploadResponse, resp); err != nil {
		return err
	}

	c.ui.Normal().Msg("Stage application ...")

	details.Info("staging code")
	out, err := json.Marshal(resp.App)
	if err != nil {
		return errors.Wrap(err, "can't marshall upload response")
	}

	_, err = c.post(api.Routes.Path("AppStage", c.Config.Org, app), string(out))
	if err != nil {
		return errors.Wrap(err, "can't stage app")
	}

	details.Info("start tailing logs")
	stopFunc, err := c.logs(app, c.Config.Org)
	if err != nil {
		return errors.Wrap(err, "failed to tail logs")
	}
	defer stopFunc()

	details.Info("wait for app")
	err = c.waitForApp(c.Config.Org, app)
	if err != nil {
		return errors.Wrap(err, "waiting for app failed")
	}

	if len(services) > 0 {
		c.ui.Note().Msg("Binding Services")

		// Application is up, bind the services.
		// This will restart the application.
		// TODO: See #347 for future work

		request := models.BindRequest{
			Names: services,
		}

		js, err := json.Marshal(request)
		if err != nil {
			return err
		}

		_, err = c.post(api.Routes.Path("ServiceBindingCreate", c.Config.Org, app), string(js))
		if err != nil {
			return err
		}

		c.ui.Note().Msg("Done")
	}

	c.ui.Success().
		WithStringValue("Name", app).
		WithStringValue("Organization", c.Config.Org).
		WithStringValue("Route", fmt.Sprintf("https://%s", resp.App.Route)).
		Msg("App is online.")

	return nil
}

// Target targets an org in gitea
func (c *EpinioClient) Target(org string) error {
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

func (c *EpinioClient) check() {
	c.GiteaClient.Client.GetMyUserInfo()
}

func (c *EpinioClient) deleteLocalCertificate(appName string) error {
	quarksSecretInstanceGVR := schema.GroupVersionResource{
		Group:    "quarks.cloudfoundry.org",
		Version:  "v1alpha1",
		Resource: "quarkssecrets",
	}

	dynamicClient, err := dynamic.NewForConfig(c.KubeClient.RestConfig)
	if err != nil {
		return err
	}

	err = dynamicClient.Resource(quarksSecretInstanceGVR).Namespace(c.Config.Org).
		Delete(context.Background(), appName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *EpinioClient) deleteProductionCertificate(appName string) error {
	certificateInstanceGVR := schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1alpha2",
		Resource: "certificates",
	}

	dynamicClient, err := dynamic.NewForConfig(c.KubeClient.RestConfig)
	if err != nil {
		return err
	}

	err = dynamicClient.Resource(certificateInstanceGVR).Namespace(c.Config.Org).
		Delete(context.Background(), appName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	err = c.KubeClient.Kubectl.CoreV1().Secrets(c.Config.Org).Delete(context.Background(), fmt.Sprintf("%s-tls", appName), metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (c *EpinioClient) logs(appName, org string) (context.CancelFunc, error) {
	c.ui.ProgressNote().V(1).Msg("Tailing application logs ...")

	ctx, cancelFunc := context.WithCancel(context.Background())

	selector := labels.NewSelector()

	for _, req := range [][]string{
		{"app.kubernetes.io/managed-by", "epinio"},
		{"app.kubernetes.io/part-of", org},
		{"app.kubernetes.io/name", appName},
	} {
		req, err := labels.NewRequirement(req[0], selection.Equals, []string{req[1]})
		if err != nil {
			return cancelFunc, err
		}
		selector = selector.Add(*req)
	}

	err := tailer.Run(c.ui, ctx, &tailer.Config{
		ContainerQuery:        regexp.MustCompile(".*"),
		ExcludeContainerQuery: nil,
		ContainerState:        "running",
		Exclude:               nil,
		Include:               nil,
		Timestamps:            false,
		Since:                 duration.LogHistory(),
		AllNamespaces:         true,
		LabelSelector:         selector,
		TailLines:             nil,
		Template:              tailer.DefaultSingleNamespaceTemplate(),
		Namespace:             "",
		PodQuery:              regexp.MustCompile(".*"),
	}, c.KubeClient)
	if err != nil {
		return cancelFunc, errors.Wrap(err, "failed to start log tail")
	}

	return cancelFunc, nil
}

func (c *EpinioClient) waitForApp(org, name string) error {
	c.ui.ProgressNote().KeeplineUnder(1).Msg("Creating application resources")
	err := c.KubeClient.WaitUntilPodBySelectorExist(
		c.ui, org, fmt.Sprintf("app.kubernetes.io/name=%s", name),
		duration.ToAppBuilt())
	if err != nil {
		return errors.Wrap(err, "waiting for app to be created failed")
	}

	c.ui.ProgressNote().KeeplineUnder(1).Msg("Starting application")

	err = c.KubeClient.WaitForPodBySelectorRunning(
		c.ui, org, fmt.Sprintf("app.kubernetes.io/name=%s", name),
		duration.ToPodReady())

	if err != nil {
		return errors.Wrap(err, "waiting for app to come online failed")
	}

	return nil
}

// TODO: Delete after all commands go through the api
func (c *EpinioClient) ensureGoodOrg(org, msg string) error {
	_, resp, err := c.GiteaClient.Client.GetOrg(org)
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

func (c *EpinioClient) ServicesToApps(org string) (map[string]application.ApplicationList, error) {
	// Determine apps bound to services
	// (inversion of services bound to apps)
	// Literally query apps in the org for their services and invert.

	var appsOf = map[string]application.ApplicationList{}

	apps, err := application.List(c.KubeClient, c.Config.Org)
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

func (c *EpinioClient) get(endpoint string) ([]byte, error) {
	return c.curl(endpoint, "GET", "")
}

func (c *EpinioClient) post(endpoint string, data string) ([]byte, error) {
	return c.curl(endpoint, "POST", data)
}

func (c *EpinioClient) delete(endpoint string) ([]byte, error) {
	return c.curl(endpoint, "DELETE", "")
}

// upload the given path as param "file" in a multipart form
func (c *EpinioClient) upload(endpoint string, path string) ([]byte, error) {
	uri := fmt.Sprintf("%s/%s", c.serverURL, endpoint)

	// open the tarball
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open tarball")
	}
	defer file.Close()

	// create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(file.Name()))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create multiform part")
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write to multiform part")
	}

	err = writer.Close()
	if err != nil {
		return nil, errors.Wrap(err, "failed to close multiform")
	}

	// make the request
	request, err := http.NewRequest("POST", uri, body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build request")
	}
	request.Header.Add("Content-Type", writer.FormDataContentType())

	response, err := (&http.Client{}).Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to POST to upload")
	}
	defer response.Body.Close()

	bodyBytes, _ := ioutil.ReadAll(response.Body)
	if response.StatusCode == http.StatusCreated {
		return bodyBytes, nil
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server status code: %s\n%s", http.StatusText(response.StatusCode), string(bodyBytes))
	}

	// object was not created, but status was ok?
	return bodyBytes, nil
}

func (c *EpinioClient) curl(endpoint, method, requestBody string) ([]byte, error) {
	uri := fmt.Sprintf("%s/%s", c.serverURL, endpoint)
	c.Log.Info(fmt.Sprintf("%s %s", method, uri))
	c.Log.V(1).Info(requestBody)
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

	if response.StatusCode == http.StatusCreated {
		return bodyBytes, nil
	}

	if response.StatusCode != http.StatusOK {
		return []byte{}, errors.New(fmt.Sprintf("%s: %s", http.StatusText(response.StatusCode), string(bodyBytes)))
	}

	return bodyBytes, nil
}

func (c *EpinioClient) curlWithCustomErrorHandling(endpoint, method, requestBody string,
	f func(response *http.Response, bodyBytes []byte, err error) error) ([]byte, error) {

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

	if response.StatusCode == http.StatusCreated {
		return bodyBytes, nil
	}

	if response.StatusCode != http.StatusOK {
		return []byte{}, f(response, bodyBytes,
			errors.New(fmt.Sprintf("%s: %s", http.StatusText(response.StatusCode), string(bodyBytes))))
	}

	return bodyBytes, nil
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
