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

	eiriniclient "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned"
	"code.gitea.io/sdk/gitea"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/kubernetes/tailer"
	"github.com/suse/carrier/cli/paas/config"
	paasgitea "github.com/suse/carrier/cli/paas/gitea"
	"github.com/suse/carrier/cli/paas/ui"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	// HookSecret should be generated
	// TODO: generate this and put it in a secret
	HookSecret = "74tZTBHkhjMT5Klj6Ik6PqmM"

	// StagingEventListenerURL should not exist
	// TODO: detect this based on namespaces and services
	StagingEventListenerURL = "http://el-staging-listener.eirini-workloads:8080"
)

// CarrierClient provides functionality for talking to a
// Carrier installation on Kubernetes
type CarrierClient struct {
	giteaClient   *gitea.Client
	kubeClient    *kubernetes.Cluster
	ui            *ui.UI
	config        *config.Config
	giteaResolver *paasgitea.Resolver
	eiriniClient  *eiriniclient.Clientset
}

// Info displays information about environment
func (c *CarrierClient) Info() error {
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
	result := []string{}

	apps, _, err := c.giteaClient.ListOrgRepos(c.config.Org, gitea.ListOrgReposOptions{})
	if err != nil {
		return result
	}

	for _, app := range apps {
		if strings.HasPrefix(app.Name, prefix) {
			result = append(result, app.Name)
		}
	}

	return result
}

// Apps gets all Carrier apps
func (c *CarrierClient) Apps() error {
	apps, _, err := c.giteaClient.ListOrgRepos(c.config.Org, gitea.ListOrgReposOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list apps")
	}

	msg := c.ui.Normal().WithTable("Name", "Status", "Routes")

	for _, app := range apps {
		status, err := c.kubeClient.StatefulSetStatus(
			c.config.EiriniWorkloadsNamespace,
			fmt.Sprintf("cloudfoundry.org/guid=%s", app.Name))
		if err != nil {
			return errors.Wrapf(err, "failed to get status for app '%s'", app.Name)
		}

		routes, err := c.kubeClient.ListIngressRoutes(
			c.config.EiriniWorkloadsNamespace,
			app.Name)
		if err != nil {
			return errors.Wrapf(err, "failed to get routes for app '%s'", app.Name)
		}

		msg = msg.WithTableRow(app.Name, status, strings.Join(routes, ", "))
	}

	msg.Msg("Carrier Apps.")

	return nil
}

// CreateOrg creates an Org in gitea
func (c *CarrierClient) CreateOrg(org string) error {
	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Creating organization...")

	_, resp, err := c.giteaClient.GetOrg(org)
	if resp == nil && err != nil {
		return errors.Wrap(err, "failed to make get org request")
	}

	if resp.StatusCode == 200 {
		c.ui.Exclamation().Msg("Organization already exists.")
		return nil
	}

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
	c.ui.Note().
		WithStringValue("Name", app).
		Msg("Deleting application...")

	_, err := c.giteaClient.DeleteRepo(c.config.Org, app)
	if err != nil {
		return errors.Wrap(err, "failed to delete repo")
	}

	c.ui.Normal().Msg("Deleted app code repository.")

	err = c.eiriniClient.EiriniV1().LRPs(c.config.EiriniWorkloadsNamespace).Delete(context.Background(), app, metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to delete eirini lrp")
	}

	c.ui.Normal().Msg("Deleted app containers.")
	c.ui.Success().Msg("Application deleted.")

	return nil
}

// OrgsMatching returns all Carrier orgs having the specified prefix
// in their name
func (c *CarrierClient) OrgsMatching(prefix string) []string {
	result := []string{}

	orgs, _, err := c.giteaClient.AdminListOrgs(gitea.AdminListOrgsOptions{})
	if err != nil {
		return result
	}

	for _, org := range orgs {
		if strings.HasPrefix(org.UserName, prefix) {
			result = append(result, org.UserName)
		}
	}

	return result
}

// Orgs get a list of all orgs in gitea
func (c *CarrierClient) Orgs() error {
	orgs, _, err := c.giteaClient.AdminListOrgs(gitea.AdminListOrgsOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list orgs")
	}

	msg := c.ui.Normal().WithTable("Name")

	for _, org := range orgs {
		msg = msg.WithTableRow(org.UserName)
	}

	msg.Msg("Carrier Organizations")

	return nil
}

// Push pushes an app
func (c *CarrierClient) Push(app string, path string) error {
	c.ui.Note().
		WithStringValue("Name", app).
		WithStringValue("Organization", c.config.Org).
		WithStringValue("Sources", path).
		Msg("Pushing application")

	err := c.createRepo(app)
	if err != nil {
		return errors.Wrap(err, "create repo failed")
	}

	err = c.createRepoWebhook(app)
	if err != nil {
		return errors.Wrap(err, "webhook configuration failed")
	}

	tmpDir, err := c.prepareCode(app, path)
	if err != nil {
		return errors.Wrap(err, "failed to prepare code")
	}

	err = c.gitPush(app, tmpDir)
	if err != nil {
		return errors.Wrap(err, "failed to git push code")
	}

	stopFunc, err := c.logs(app)
	if err != nil {
		return errors.Wrap(err, "failed to tail logs")
	}
	defer stopFunc()

	err = c.waitForApp(c.ui, app)
	if err != nil {
		return errors.Wrap(err, "waiting for app failed")
	}

	route, err := c.appDefaultRoute(app)
	if err != nil {
		return errors.Wrap(err, "failed to determine default app route")
	}

	c.ui.Success().
		WithStringValue("Name", app).
		WithStringValue("Organization", c.config.Org).
		WithStringValue("Route", fmt.Sprintf("http://%s", route)).
		Msg("App is online.")

	return nil
}

// Target targets an org in gitea
func (c *CarrierClient) Target(org string) error {
	if org == "" {
		c.ui.Success().
			WithStringValue("Currently targeted organization", c.config.Org).
			Msg("")
		return nil
	}

	c.ui.Note().
		WithStringValue("Name", org).
		Msg("Targeting organization...")

	_, resp, err := c.giteaClient.GetOrg(org)
	if resp == nil && err != nil {
		return errors.Wrap(err, "failed to make get org request")
	}

	if resp.StatusCode == 404 {
		c.ui.Exclamation().Msg("Organization does not exist.")
		return nil
	}

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
	c.ui.Normal().Msg("Creating webhook in the repo ...")

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

func (c *CarrierClient) prepareCode(name, appDir string) (tmpDir string, err error) {
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

	// TODO: perhaps marshaling an actual struct is better
	// TODO: name of LRP will lead to collisions, since the same app name
	// can be present in more than one organization
	lrpTmpl, err := template.New("lrp").Parse(`
---
apiVersion: eirini.cloudfoundry.org/v1
kind: LRP
metadata:
  name: "{{ .AppName }}"
  namespace: eirini-workloads
spec:
  GUID: "{{ .AppName }}"
  version: "version-1"
  appName: "{{ .AppName }}"
  instances: 1
  lastUpdated: "never"
  diskMB: 100
  runsAsRoot: true
  env:
    PORT: "8080"
  ports:
  - 8080
  image: "127.0.0.1:30500/apps/{{ .AppName }}"
  appRoutes:
  - hostname: "{{ .Route }}"
    port: 8080
`)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse lrp template for app")
	}

	appFile, err := os.Create(filepath.Join(tmpDir, ".kube", "app.yml"))
	if err != nil {
		return "", errors.Wrap(err, "failed to create file for kube resource definitions")
	}
	defer func() { err = appFile.Close() }()

	err = lrpTmpl.Execute(appFile, struct {
		AppName string
		Route   string
	}{
		AppName: name,
		Route:   route,
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

	c.ui.Success().
		WithStringValue("Output", string(output)).
		Msg("Application push successful")

	return nil
}

func (c *CarrierClient) logs(name string) (context.CancelFunc, error) {
	c.ui.ProgressNote().Msg("Tailing application logs ...")

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

		Namespace: "eirini-workloads",
		PodQuery:  regexp.MustCompile(fmt.Sprintf(".*-%s-.*", name)),
	}, c.kubeClient)
	if err != nil {
		return cancelFunc, errors.Wrap(err, "failed to start log tail")
	}

	return cancelFunc, nil
}

func (c *CarrierClient) waitForApp(ui *ui.UI, name string) error {
	c.ui.ProgressNote().Msg("Creating application resources")
	err := c.kubeClient.WaitUntilPodBySelectorExist(
		ui, c.config.EiriniWorkloadsNamespace,
		fmt.Sprintf("cloudfoundry.org/guid=%s", name),
		300)

	c.ui.ProgressNote().Msg("Starting application")

	err = c.kubeClient.WaitForPodBySelectorRunning(
		ui, c.config.EiriniWorkloadsNamespace,
		fmt.Sprintf("cloudfoundry.org/guid=%s", name),
		300)

	if err != nil {
		return errors.Wrap(err, "waiting for app to come online failed")
	}

	return nil
}
