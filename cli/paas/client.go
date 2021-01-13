package paas

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/paas/config"
	paasgitea "github.com/suse/carrier/cli/paas/gitea"
	"github.com/suse/carrier/cli/paas/ui"
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

	c.ui.Normal().
		WithStringValue("Platform", platform.String()).
		WithStringValue("Kubernetes Version", kubeVersion).
		WithStringValue("Gitea Version", giteaVersion).
		Msg("Carrier Environment")

	return nil
}

// Apps gets all Carrier apps
func (c *CarrierClient) Apps() error {
	apps, _, err := c.giteaClient.ListOrgRepos(c.config.Org, gitea.ListOrgReposOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list apps")
	}

	msg := c.ui.Normal().WithTable("Name", "Status", "Route")

	for _, app := range apps {
		msg = msg.WithTableRow(app.Name)
	}

	msg.Msg("Carrier Organizations")

	return nil
}

// CreateOrg creates an Org in gitea
func (c *CarrierClient) CreateOrg(org string) error {
	_, _, err := c.giteaClient.CreateOrg(gitea.CreateOrgOption{
		Name: org,
	})

	if err != nil {
		return errors.Wrap(err, "failed to create org")
	}

	c.ui.Normal().WithStringValue("name", org).Msg("Organization created.")

	return nil
}

// Delete deletes an app
func (c *CarrierClient) Delete(app string) error {
	return nil
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

	err = c.logs(app)
	if err != nil {
		return errors.Wrap(err, "failed to tail logs")
	}

	err = c.waitForApp(app)
	if err != nil {
		return errors.Wrap(err, "waiting for app failed")
	}

	return nil
}

// Target targets an org in gitea
func (c *CarrierClient) Target(org string) error {
	if org == "" {
		c.ui.Normal().WithStringValue("Name", c.config.Org).Msg("Targetted org:")
	}

	c.config.Org = org
	err := c.config.Save()
	if err != nil {
		return errors.Wrap(err, "failed to save configuration")
	}

	c.ui.Normal().WithStringValue("Name", c.config.Org).Msg("Targetted org:")

	return nil
}

func (c *CarrierClient) check() {
	c.giteaClient.GetMyUserInfo()
}

func (c *CarrierClient) createRepo(name string) error {
	c.ui.Normal().Msg("Creating application ...")

	_, _, err := c.giteaClient.CreateOrgRepo(c.config.Org, gitea.CreateRepoOption{
		Name:          name,
		AutoInit:      true,
		Private:       true,
		DefaultBranch: "main",
	})

	if err != nil {
		return errors.Wrap(err, "failed to create application")
	}

	c.ui.Normal().WithStringValue("name", name).Msg("Application created.")

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

	// TODO: fixme
	domain := "FOO"

	// TODO: perhaps marshaling an actual struct is better
	// TODO: name of LRP will lead to collisions, since the same app name
	// can be present in more than one organization
	lrpTmpl, err := template.New("lrp").Parse(`
apiVersion: eirini.cloudfoundry.org/v1
kind: LRP
metadata:
	name: "{{ .AppName }}"
	namespace: eirini-workloads
spec:
	GUID: "${{ .AppName }}"
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
	- hostname: "{{ .AppName }}.{{ .Domain }}"
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
		Domain  string
	}{
		AppName: name,
		Domain:  domain,
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
			WithStringValue("stdout", string(output)).
			WithStringValue("stderr", "").
			Msg("App push failed")
		return errors.Wrap(err, "push script failed")
	}

	c.ui.Exclamation().WithStringValue("output", string(output)).Msg("App push successful")

	return nil
}

func (c *CarrierClient) logs(name string) error {
	c.ui.Normal().Msg("Tailing app logs ...")
	//  stern --namespace "eirini-workloads" ".*$app_name.*" &

	return nil
}

func (c *CarrierClient) waitForApp(name string) error {
	c.ui.Normal().Msg("Waiting for app to come online ...")

	err := c.kubeClient.WaitForPodBySelectorRunning(
		c.config.EiriniWorkloadsNamespace,
		fmt.Sprintf("cloudfoundry.org/guid=%s", name),
		300)

	if err != nil {
		return errors.Wrap(err, "waiting for app to come online failed")
	}

	c.ui.Exclamation().WithStringValue("App Name", name).Msg("App is online.")
	return nil
}
