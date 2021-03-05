package deployments

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"github.com/suse/carrier/helpers"
	"github.com/suse/carrier/kubernetes"
	"github.com/suse/carrier/paas/ui"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Gitea struct {
	Debug   bool
	Timeout int
}

const (
	GiteaDeploymentID = "gitea"
	giteaVersion      = "2.1.3"
	giteaChartURL     = "https://dl.gitea.io/charts/gitea-2.1.3.tgz"
)

func (k *Gitea) ID() string {
	return GiteaDeploymentID
}

func (k *Gitea) Backup(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k *Gitea) Restore(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k Gitea) Describe() string {
	return emoji.Sprintf(":cloud:Gitea version: %s\n:clipboard:Gitea chart: %s", giteaVersion, giteaChartURL)
}

// Delete removes Gitea from kubernetes cluster
func (k Gitea) Delete(c *kubernetes.Cluster, ui *ui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Gitea...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(GiteaDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", GiteaDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Gitea because namespace either doesn't exist or not owned by Carrier")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling Gitea: " + err.Error())
	}

	message := "Removing helm release " + GiteaDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			helmCmd := fmt.Sprintf("helm uninstall gitea --namespace %s", GiteaDeploymentID)
			return helpers.RunProc(helmCmd, currentdir, k.Debug)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", GiteaDeploymentID)
		} else {
			return errors.Wrapf(err, "Failed uninstalling helm release %s: %s", GiteaDeploymentID, out)
		}
	}

	message = "Deleting Gitea namespace " + GiteaDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(GiteaDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", GiteaDeploymentID)
	}

	ui.Success().Msg("Gitea removed")

	return nil
}

func (k Gitea) apply(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Setup Gitea helm values
	var helmArgs []string

	domain, err := options.GetString("system_domain", GiteaDeploymentID)
	if err != nil {
		return err
	}
	subdomain := GiteaDeploymentID + "." + domain

	config := fmt.Sprintf(`
ingress:
  enabled: true
  hosts:
    - %s
  annotations:
    kubernetes.io/ingress.class: traefik
service:
  http:
    type: NodePort
    port: 10080
  ssh:
    type: NodePort
    port: 10022
  externalTrafficPolicy: Local

gitea:
  admin:
    username: "dev"
    password: "changeme"
    email: "admin@carrier.sh"
  config:
    APP_NAME: "Carrier"
    RUN_MODE: prod
    repository:
      ROOT:  "/data/git/gitea-repositories"
    database:
      DB_TYPE: sqlite3
      PATH: /data/gitea/gitea.db
    server:
      DOMAIN:  %s
      ROOT_URL: %s
    security:
      INSTALL_LOCK: true
      SECRET_KEY: generated-by-quarks-secret
      INTERNAL_TOKEN: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYmYiOjE2MDIzNzc3NzZ9.uvJPCMSDTPlVMAUwNzW9Jbl5487kbj5T_pWu3dGirnA
    service:
      ENABLE_REGISTRATION_CAPTCHA: false
      DISABLE_REGISTRATION: true
    openid:
      ENABLE_OPENID_SIGNIN: false
    oauth2:
      ENABLE: true
      JWT_SECRET: HLNn92qqtznZSMkD_TzR_XFVdiZ5E87oaus6pyH7tiI
`, subdomain, subdomain, "http://"+subdomain)

	configPath, err := helpers.CreateTmpFile(config)
	if err != nil {
		return err
	}
	defer os.Remove(configPath)

	helmCmd := fmt.Sprintf("helm %s gitea --create-namespace --values %s --namespace %s %s %s", action, configPath, GiteaDeploymentID, giteaChartURL, strings.Join(helmArgs, " "))

	if out, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.New("Failed installing Gitea: " + out)
	}
	err = c.LabelNamespace(GiteaDeploymentID, kubernetes.CarrierDeploymentLabelKey, kubernetes.CarrierDeploymentLabelValue)
	if err != nil {
		return err
	}

	for _, podname := range []string{
		"memcached",
		"postgresql",
		"gitea",
	} {
		if err := c.WaitUntilPodBySelectorExist(ui, GiteaDeploymentID, "app.kubernetes.io/name="+podname, k.Timeout); err != nil {
			return errors.Wrap(err, "failed waiting Gitea "+podname+" deployment to exist")
		}
		if err := c.WaitForPodBySelectorRunning(ui, GiteaDeploymentID, "app.kubernetes.io/name="+podname, k.Timeout); err != nil {
			return errors.Wrap(err, "failed waiting Gitea "+podname+" deployment to come up")
		}
	}

	ui.Success().Msg("Gitea deployed")

	return nil
}

func (k Gitea) GetVersion() string {
	return giteaVersion
}

func (k Gitea) Deploy(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		GiteaDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + GiteaDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Gitea...")

	err = k.apply(c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Gitea) Upgrade(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		GiteaDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + GiteaDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Gitea...")

	return k.apply(c, ui, options, true)
}
