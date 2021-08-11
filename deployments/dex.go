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
	"github.com/epinio/epinio/internal/duration"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Dex struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &Dex{}

const (
	DexDeploymentID = "dex"
	dexVersion      = "2.30.0"
	dexChartFile    = "dex-0.6.0.tgz"
)

func (k *Dex) ID() string {
	return DexDeploymentID
}

func (k Dex) Describe() string {
	return emoji.Sprintf(":cloud:Dex version: %s\n", dexVersion)
}

// Delete removes Dex from kubernetes cluster
func (k Dex) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Dex...")

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, DexDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", DexDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Dex because namespace either doesn't exist or not owned by Dex")
		return nil
	}

	message := "Removing helm release " + DexDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.RunProc(currentdir, k.Debug,
				"helm", "uninstall", DexDeploymentID, "--namespace", DexDeploymentID)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", DexDeploymentID)
		} else {
			return errors.Wrapf(err, "Failed uninstalling helm release %s: %s", DexDeploymentID, out)
		}
	}

	message = "Deleting Dex namespace " + DexDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ctx, DexDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", DexDeploymentID)
	}

	err = c.WaitForNamespaceMissing(ctx, ui, DexDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	ui.Success().Msg("Dex removed")

	return nil
}

func (k Dex) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	if err := c.CreateNamespace(ctx, DexDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	domain, err := options.GetString("system_domain", TektonDeploymentID)
	if err != nil {
		return errors.Wrap(err, "Couldn't get system_domain option")
	}

	issuer := options.GetStringNG("tls-issuer")

	// Wait for the cert manager to be present and active. It is required
	if err := waitForCertManagerReady(ctx, ui, c, issuer); err != nil {
		return errors.Wrap(err, "waiting for cert-manager failed")
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	tarPath, err := helpers.ExtractFile(dexChartFile)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tarPath + " - " + err.Error())
	}
	defer os.Remove(tarPath)

	// We wait for Traefik Forward Auth client secret
	// to be created so that we can use the values of the
	// secret in the dex config.
	if err := c.WaitForNamespace(ctx, ui, TraefikForwardAuthDeploymentID, duration.ToDeploymentNamespaceCreated()); err != nil {
		return errors.Wrapf(err, "waiting for %s deployment", TraefikForwardAuthDeploymentID)
	}
	traefikAuthClientSecretName := fmt.Sprintf("%s-client", TraefikForwardAuthDeploymentID)
	traefikAuthClientSecret, err := c.WaitForSecret(ctx, TraefikForwardAuthDeploymentID, traefikAuthClientSecretName, duration.ToAuthClientSecretCreated())
	if err != nil {
		return errors.Wrapf(err, "waiting for %s secret", traefikAuthClientSecretName)
	}

	username, err := options.GetString("user", "")
	if err != nil {
		return err
	}

	password, err := options.GetString("password", "")
	if err != nil {
		return err
	}

	passwordHash, err := auth.HashBcrypt(password)
	if err != nil {
		return errors.Wrap(err, "generating hash for api password")
	}

	k.createStaticUserSecret(ctx, c, username, password)

	// https://github.com/dexidp/dex/blob/master/config.yaml.dist
	config := fmt.Sprintf(`
issuer: https://%[6]s

https:
  port: 5554

ingress:
  enabled: true

  annotations:
    kubernetes.io/ingress.class: "traefik"
    cert-manager.io/cluster-issuer: %[1]s
    traefik.ingress.kubernetes.io/router.entrypoints: websecure
    traefik.ingress.kubernetes.io/router.tls: "true"
  hosts:
    - host: %[2]s
      paths:
        - path: /
          pathType: Prefix

  tls:
    - hosts:
        - %[2]s
      secretName: dex-cert
config:
  issuer: https://%[6]s

  storage:
    type: kubernetes
    config:
      inCluster: true

  web:
    http: %[6]s

  enablePasswordDB: true

  staticPasswords:
    - email: "%[7]s"
      hash: "%[8]s"
      username: "%[7]s"
      userID: "1"

  staticClients:
    - id: %[3]s
      secret: %[4]s
      name: 'Epinio'
      # Where the app will be running.
      redirectURIs:
      - '%[5]s'
`,
		issuer,
		DexDeploymentID+"."+domain,
		traefikAuthClientSecret.Data["username"],
		traefikAuthClientSecret.Data["password"],
		fmt.Sprintf("https://auth.%s/_oauth", domain),
		fmt.Sprintf("%s.%s", DexDeploymentID, domain),
		username, passwordHash)

	configPath, err := helpers.CreateTmpFile(config)
	if err != nil {
		return err
	}
	defer os.Remove(configPath)

	helmArgs := []string{
		action, DexDeploymentID, "--values", configPath, "--namespace", DexDeploymentID, tarPath,
	}
	if out, err := helpers.RunProc(currentdir, k.Debug, "helm", helmArgs...); err != nil {
		return errors.New("Failed installing Dex: " + out)
	}

	if err := c.WaitUntilPodBySelectorExist(ctx, ui, DexDeploymentID, "app.kubernetes.io/name=dex", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Dex deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning(ctx, ui, DexDeploymentID, "app.kubernetes.io/name=dex", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Dex deployment to come up")
	}

	ui.Success().Msg("Dex deployed")

	return nil
}

func (k Dex) GetVersion() string {
	return dexVersion
}

func (k Dex) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

func (k Dex) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(ctx, DexDeploymentID, metav1.GetOptions{})
	if err == nil {
		return errors.New("Namespace " + DexDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Dex...")

	err = k.apply(ctx, c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Dex) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		DexDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + DexDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Dex...")

	return k.apply(ctx, c, ui, options, true)
}

func (k Dex) createStaticUserSecret(ctx context.Context, c *kubernetes.Cluster, username, password string) error {
	_, err := c.Kubectl.CoreV1().Secrets(DexDeploymentID).Create(ctx,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dex-static-user",
			},
			StringData: map[string]string{
				"username": username,
				"password": password,
			},
			Type: "Opaque",
		}, metav1.CreateOptions{})

	return err
}
