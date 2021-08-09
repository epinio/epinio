package deployments

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/auth"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Gitea struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &Gitea{}

const (
	GiteaProtocol     = "http"
	GiteaPort         = "10080"
	GiteaServiceName  = "gitea-http"
	GiteaDeploymentID = "gitea"
	GiteaURL          = GiteaProtocol + "://" + GiteaServiceName + "." + GiteaDeploymentID + ":" + GiteaPort
	giteaVersion      = "2.1.3"
	giteaChartURL     = "https://dl.gitea.io/charts/gitea-2.1.3.tgz"
)

var giteaAuthMemo *auth.PasswordAuth

func GiteaInstallAuth() (*auth.PasswordAuth, error) {
	if giteaAuthMemo == nil {
		auth, err := auth.RandomPasswordAuth()
		if err != nil {
			return nil, err
		}
		giteaAuthMemo = auth
	}
	return giteaAuthMemo, nil
}

func (k Gitea) ID() string {
	return GiteaDeploymentID
}

func (k Gitea) Describe() string {
	return emoji.Sprintf(":cloud:Gitea version: %s\n:clipboard:Gitea chart: %s", giteaVersion, giteaChartURL)
}

func (k Gitea) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

// Delete removes Gitea from kubernetes cluster
func (k Gitea) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Gitea...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, GiteaDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", GiteaDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Gitea because namespace either doesn't exist or not owned by Epinio")
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
			return "", c.DeleteNamespace(ctx, GiteaDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", GiteaDeploymentID)
	}

	ui.Success().Msg("Gitea removed")

	return nil
}

func (k Gitea) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	if err := c.CreateNamespace(ctx, GiteaDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
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

	// See deployments/tekton.go, func `createGiteaCredsSecret`
	// for where `install` configures tekton for the same
	// gitea.admin credentials.
	//
	// See internal/cli/clients/gitea/gitea.go, func
	// `getGiteaCredentials` for where the cli retrieves the
	// information for its own gitea client.
	giteaAuth, err := GiteaInstallAuth()
	if err != nil {
		return err
	}

	config := fmt.Sprintf(`
ingress:
  enabled: false
  hosts:
    - %s
  annotations:
    kubernetes.io/ingress.class: traefik
service:
  http:
    type: NodePort
    port: %s
  ssh:
    type: NodePort
    port: 10022
  externalTrafficPolicy: Local

gitea:
  admin:
    username: "%s"
    password: "%s"
    email: "admin@epinio.sh"
  config:
    APP_NAME: "Epinio"
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
    service:
      ENABLE_REGISTRATION_CAPTCHA: false
      DISABLE_REGISTRATION: true
    openid:
      ENABLE_OPENID_SIGNIN: false
    oauth2:
      ENABLE: false

postgresql:
  global:
    postgresql:
      postgresqlDatabase: epinio-gitea
      postgresqlUsername: "%s"
      postgresqlPassword: "%s"
`, subdomain, GiteaPort,
		giteaAuth.Username, giteaAuth.Password,
		subdomain, GiteaProtocol+"://"+subdomain,
		giteaAuth.Username, giteaAuth.Password)

	configPath, err := helpers.CreateTmpFile(config)
	if err != nil {
		return err
	}
	defer os.Remove(configPath)

	helmCmd := fmt.Sprintf("helm %s gitea --values %s --namespace %s %s %s", action, configPath, GiteaDeploymentID, giteaChartURL, strings.Join(helmArgs, " "))

	if out, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.New("Failed installing Gitea: " + out)
	}

	for _, podname := range []string{
		"memcached",
		"postgresql",
		"gitea",
	} {
		if err := c.WaitUntilPodBySelectorExist(ctx, ui, GiteaDeploymentID, "app.kubernetes.io/name="+podname, k.Timeout); err != nil {
			return errors.Wrap(err, "failed waiting Gitea "+podname+" deployment to exist")
		}
		if err := c.WaitForPodBySelectorRunning(ctx, ui, GiteaDeploymentID, "app.kubernetes.io/name="+podname, k.Timeout); err != nil {
			return errors.Wrap(err, "failed waiting Gitea "+podname+" deployment to come up")
		}
	}

	ui.Success().Msg("Gitea deployed")

	return nil
}

func (k Gitea) GetVersion() string {
	return giteaVersion
}

func (k Gitea) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		GiteaDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + GiteaDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Gitea...")

	err = k.apply(ctx, c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Gitea) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		GiteaDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + GiteaDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Gitea...")

	return k.apply(ctx, c, ui, options, true)
}
