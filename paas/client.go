package paas

import (
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/fatih/color"
	"github.com/go-logr/logr"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/suse/carrier/deployments"
	"github.com/suse/carrier/internal/application"
	"github.com/suse/carrier/internal/services"
	"github.com/suse/carrier/kubernetes"
	kubeconfig "github.com/suse/carrier/kubernetes/config"
	"github.com/suse/carrier/kubernetes/tailer"
	"github.com/suse/carrier/paas/config"
	paasgitea "github.com/suse/carrier/paas/gitea"
	"github.com/suse/carrier/paas/ui"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	// HookSecret should be generated
	// TODO: generate this and put it in a secret
	HookSecret = "74tZTBHkhjMT5Klj6Ik6PqmM"

	// StagingEventListenerURL should not exist
	// TODO: detect this based on namespaces and services
	StagingEventListenerURL = "http://el-staging-listener.carrier-workloads:8080"
)

// CarrierClient provides functionality for talking to a
// Carrier installation on Kubernetes
type CarrierClient struct {
	giteaClient   *gitea.Client
	kubeClient    *kubernetes.Cluster
	ui            *ui.UI
	config        *config.Config
	giteaResolver *paasgitea.Resolver
	Log           logr.Logger
}

func NewCarrierClient(flags *pflag.FlagSet) (*CarrierClient, func(), error) {
	configConfig, err := config.Load(flags)
	if err != nil {
		return nil, nil, err
	}
	restConfig, err := kubeconfig.KubeConfig()
	if err != nil {
		return nil, nil, err
	}
	cluster, err := kubernetes.NewClusterFromClient(restConfig)
	if err != nil {
		return nil, nil, err
	}
	resolver := paasgitea.NewResolver(configConfig, cluster)
	client, err := paasgitea.NewGiteaClient(resolver)
	if err != nil {
		return nil, nil, err
	}
	uiUI := ui.NewUI()
	logger := kubeconfig.NewClientLogger()
	carrierClient := &CarrierClient{
		giteaClient:   client,
		kubeClient:    cluster,
		ui:            uiUI,
		config:        configConfig,
		giteaResolver: resolver,
		Log:           logger,
	}
	return carrierClient, func() {
	}, nil
}

// Services gets all Carrier services in the targeted org
func (c *CarrierClient) Services() error {
	log := c.Log.WithName("Services").WithValues("Organization", c.config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Organization", c.config.Org).
		Msg("Listing services")

	details.Info("validate")
	err := c.ensureGoodOrg(c.config.Org, "Unable to list applications.")
	if err != nil {
		return err
	}

	orgServices, err := services.List(c.kubeClient, c.config.Org)
	if err != nil {
		return errors.Wrap(err, "failed to list services")
	}

	details.Info("list service secrets")

	msg := c.ui.Success().WithTable("Name")
	for _, s := range orgServices {
		msg = msg.WithTableRow(s.Name())
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

	orgServices, err := services.List(c.kubeClient, c.config.Org)
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

// BindService deletes a service specified by name
func (c *CarrierClient) BindService(serviceName, appName string) error {
	log := c.Log.WithName("Bind Service").
		WithValues("Name", serviceName, "Application", appName, "Organization", c.config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Organization", c.config.Org).
		Msg("Bind Service")

	details.Info("validate")
	err := c.ensureGoodOrg(c.config.Org, "Unable to bind service.")
	if err != nil {
		return err
	}

	// Lookup app and service. Conversion from name to internal objects.

	app, err := application.Lookup(c.kubeClient, c.giteaClient, c.config.Org, appName)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	service, err := services.Lookup(c.kubeClient, c.config.Org, serviceName)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	// Do the task

	err = service.Bind(app)

	if err != nil {
		return errors.Wrap(err, "failed to bind service")
	}

	c.ui.Success().
		WithStringValue("Service", serviceName).
		WithStringValue("Application", appName).
		WithStringValue("Organization", c.config.Org).
		Msg("Service Bound.")
	return nil
}

// DeleteService deletes a service specified by name
func (c *CarrierClient) DeleteService(name string) error {
	log := c.Log.WithName("Delete Service").
		WithValues("Name", name, "Organization", c.config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.config.Org).
		Msg("Delete Service")

	details.Info("validate")
	err := c.ensureGoodOrg(c.config.Org, "Unable to remove service.")
	if err != nil {
		return err
	}

	service, err := services.Lookup(c.kubeClient, c.config.Org, name)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	// TODO: Validation. Prevent removal of a service still bound
	// TODO: to applications.

	err = service.Delete()
	if err != nil {
		return errors.Wrap(err, "failed to delete service")
	}

	c.ui.Success().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.config.Org).
		Msg("Service Removed.")
	return nil
}

// CreateCustomService creates a service specified by name and key/value dictionary
// TODO: Allow underscores in service names (right now they fail because of kubernetes naming rules for secrets)
func (c *CarrierClient) CreateCustomService(name string, dict []string) error {
	log := c.Log.WithName("Create Custom Service").
		WithValues("Name", name, "Organization", c.config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	data := make(map[string]string)
	msg := c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Organization", c.config.Org).
		WithTable("Parameter", "Value")
	for i := 0; i < len(dict); i += 2 {
		key := dict[i]
		value := dict[i+1]
		msg = msg.WithTableRow(key, value)
		data[key] = value
	}
	msg.Msg("Create Custom Service")

	details.Info("validate")
	err := c.ensureGoodOrg(c.config.Org, "Unable to create service.")
	if err != nil {
		return err
	}

	service, err := services.CreateCustomService(c.kubeClient, name, c.config.Org, data)
	if err != nil {
		return errors.Wrap(err, "failed to create secret")
	}

	c.ui.Success().
		WithStringValue("Name", service.Name()).
		WithStringValue("Organization", service.Org()).
		Msg("Service Saved.")
	return nil
}

// Info displays information about environment
func (c *CarrierClient) Info() error {
	log := c.Log.WithName("Info")
	log.Info("start")
	defer log.Info("return")

	platform := c.kubeClient.GetPlatform()
	kubeVersion, err := c.kubeClient.GetVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get kube version")
	}

	giteaVersion := "unavailable"

	version, resp, err := c.giteaClient.ServerVersion()
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
	// TODO: change to use application.List()

	log := c.Log.WithName("AppsMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	result := []string{}

	apps, _, err := c.giteaClient.ListOrgRepos(c.config.Org, gitea.ListOrgReposOptions{})
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
	// TODO: change to use application.List()

	log := c.Log.WithName("Apps").WithValues("Organization", c.config.Org)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Organization", c.config.Org).
		Msg("Listing applications")

	details.Info("validate")
	err := c.ensureGoodOrg(c.config.Org, "Unable to list applications.")
	if err != nil {
		return err
	}

	details.Info("gitea list org repos")
	apps, _, err := c.giteaClient.ListOrgRepos(c.config.Org, gitea.ListOrgReposOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list apps")
	}

	msg := c.ui.Success().WithTable("Name", "Status", "Routes")

	for _, app := range apps {
		details.Info("kube get status", "App", app.Name)
		status, err := c.kubeClient.DeploymentStatus(
			c.config.CarrierWorkloadsNamespace,
			fmt.Sprintf("carrier/app-guid=%s.%s", c.config.Org, app.Name),
		)
		if err != nil {
			status = color.RedString(err.Error())
		}

		details.Info("kube get ingress", "App", app.Name)
		routes, err := c.kubeClient.ListIngressRoutes(
			c.config.CarrierWorkloadsNamespace,
			app.Name)
		if err != nil {
			routes = []string{color.RedString(err.Error())}
		}

		msg = msg.WithTableRow(app.Name, status, strings.Join(routes, ", "))
	}

	msg.Msg("Carrier Applications:")

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
	_, resp, err := c.giteaClient.GetOrg(org)
	if resp == nil && err != nil {
		return errors.Wrap(err, "failed to make get org request")
	}

	if resp.StatusCode == 200 {
		c.ui.Exclamation().Msg("Organization already exists.")
		return nil
	}

	details.Info("gitea create-org")
	_, _, err = c.giteaClient.CreateOrg(gitea.CreateOrgOption{
		Name: org,
	})

	if err != nil {
		return errors.Wrap(err, "failed to create org")
	}

	c.ui.Success().Msg("Organization created.")

	return nil
}

// Delete deletes an app
func (c *CarrierClient) Delete(app string) error {
	// TODO: lookup app, (get object), invoke action.
	// TODO: Move action here into `internal/application`.

	log := c.Log.WithName("Delete").WithValues("Application", app)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Name", app).
		Msg("Deleting application...")

	details.Info("delete repo")
	_, err := c.giteaClient.DeleteRepo(c.config.Org, app)
	if err != nil {
		return errors.Wrap(err, "failed to delete repository")
	}

	c.ui.Normal().Msg("Deleted application code repository.")

	details.Info("delete deployment")

	err = c.kubeClient.Kubectl.AppsV1().Deployments(c.config.CarrierWorkloadsNamespace).
		Delete(context.Background(), fmt.Sprintf("%s.%s", c.config.Org, app), metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to delete application deployment")
	}

	// The command above removes the application's deployment.
	// This in turn deletes the associated replicaset, and pod, in
	// this order. The pod being gone thus indicates command
	// completion, and is therefore what we are waiting on below.

	err = c.kubeClient.WaitForPodBySelectorMissing(c.ui,
		c.config.CarrierWorkloadsNamespace,
		fmt.Sprintf("cloudfoundry.org/guid=%s.%s", c.config.Org, app),
		DefaultTimeoutSec)
	if err != nil {
		return errors.Wrap(err, "failed to delete application pod")
	}

	c.ui.Normal().Msg("Deleted application containers.")
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

	orgs, _, err := c.giteaClient.AdminListOrgs(gitea.AdminListOrgsOptions{})
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
	orgs, _, err := c.giteaClient.AdminListOrgs(gitea.AdminListOrgsOptions{})
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
			"Organization", c.config.Org,
			"Sources", path)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Name", app).
		WithStringValue("Sources", path).
		WithStringValue("Organization", c.config.Org).
		Msg("About to push an application with given name and sources into the specified organization")

	c.ui.Exclamation().
		Timeout(5 * time.Second).
		Msg("Hit Enter to continue or Ctrl+C to abort (deployment will continue automatically in 5 seconds)")

	details.Info("validate")
	err := c.ensureGoodOrg(c.config.Org, "Unable to push.")
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
	tmpDir, err := c.prepareCode(app, c.config.Org, path)
	if err != nil {
		return errors.Wrap(err, "failed to prepare code")
	}

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

	details.Info("wait for apps")
	err = c.waitForApp(c.config.Org, app)
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
		WithStringValue("Organization", c.config.Org).
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
			WithStringValue("Currently targeted organization", c.config.Org).
			Msg("")
		return nil
	}

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Targeting organization...")

	details.Info("validate")
	err := c.ensureGoodOrg(org, "Unable to target.")
	if err != nil {
		return err
	}

	details.Info("set config")
	c.config.Org = org
	err = c.config.Save()
	if err != nil {
		return errors.Wrap(err, "failed to save configuration")
	}

	c.ui.Success().Msg("Organization targeted.")

	return nil
}

func (c *CarrierClient) check() {
	c.giteaClient.GetMyUserInfo()
}

func (c *CarrierClient) createRepo(name string) error {
	_, resp, err := c.giteaClient.GetRepo(c.config.Org, name)
	if resp == nil && err != nil {
		return errors.Wrap(err, "failed to make get repo request")
	}

	if resp.StatusCode == 200 {
		c.ui.Note().Msg("Application already exists. Updating.")
		return nil
	}

	_, _, err = c.giteaClient.CreateOrgRepo(c.config.Org, gitea.CreateRepoOption{
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
	hooks, _, err := c.giteaClient.ListRepoHooks(c.config.Org, name, gitea.ListHooksOptions{})
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

	c.giteaClient.CreateRepoHook(c.config.Org, name, gitea.CreateHookOption{
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
    carrier/app-guid:  "{{ .Org }}.{{ .AppName }}"
    carrier/app-name: "{{ .AppName }}"
    carrier/org: "{{ .Org }}"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: "{{ .AppName }}"
  template:
    metadata:
      labels:
        app: "{{ .AppName }}"
        # Needed for the ingress extension to work:
        cloudfoundry.org/guid:  "{{ .Org }}.{{ .AppName }}"
      annotations:
        # Needed for the ingress extension to work:
        cloudfoundry.org/routes: '[{ "hostname": "{{ .Route}}", "port": 8080 }]'
        cloudfoundry.org/application_name:  "{{ .AppName }}"
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
		Org:     c.config.Org,
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
	u.Path = path.Join(u.Path, c.config.Org, name)

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
git push carrier master:main
`, tmpDir, u.String(), time.Now().Format("20060102150405")))

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
		Since:                 48 * time.Hour,
		AllNamespaces:         false,
		LabelSelector:         labels.Everything(),
		TailLines:             nil,
		Template:              tailer.DefaultSingleNamespaceTemplate(),

		Namespace: "carrier-workloads",
		PodQuery:  regexp.MustCompile(fmt.Sprintf(".*-%s-.*", name)),
	}, c.kubeClient)
	if err != nil {
		return cancelFunc, errors.Wrap(err, "failed to start log tail")
	}

	return cancelFunc, nil
}

func (c *CarrierClient) waitForApp(org, name string) error {
	c.ui.ProgressNote().KeeplineUnder(1).Msg("Creating application resources")
	err := c.kubeClient.WaitUntilPodBySelectorExist(
		c.ui, c.config.CarrierWorkloadsNamespace,
		fmt.Sprintf("cloudfoundry.org/guid=%s.%s", org, name),
		300)
	if err != nil {
		return errors.Wrap(err, "waiting for app to be created failed")
	}

	c.ui.ProgressNote().KeeplineUnder(1).Msg("Starting application")

	err = c.kubeClient.WaitForPodBySelectorRunning(
		c.ui, c.config.CarrierWorkloadsNamespace,
		fmt.Sprintf("cloudfoundry.org/guid=%s.%s", org, name),
		300)

	if err != nil {
		return errors.Wrap(err, "waiting for app to come online failed")
	}

	return nil
}

func (c *CarrierClient) ensureGoodOrg(org, msg string) error {
	_, resp, err := c.giteaClient.GetOrg(org)
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
