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
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Dex struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &Dex{}

const (
	DexDeploymentID = "dex"
	dexVersion      = "2.29.0"
	dexChartFile    = "dex-0.5.0.tgz"
)

func (k *Dex) ID() string {
	return DexDeploymentID
}

func (k *Dex) Backup(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k *Dex) Restore(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
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
			helmCmd := fmt.Sprintf("helm uninstall %[1]s --namespace %[1]s", DexDeploymentID)
			return helpers.RunProc(helmCmd, currentdir, k.Debug)
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

	// Wait for the cert manager to be present and active. It is required
	waitForCertManagerReady(ctx, ui, c)

	// TODO: does the helm chart cope with cert manager not being ready?

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	tarPath, err := helpers.ExtractFile(dexChartFile)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tarPath + " - " + err.Error())
	}
	defer os.Remove(tarPath)

	issuer := options.GetStringNG("tls-issuer")

	// https://github.com/dexidp/dex/blob/master/config.yaml.dist
	config := fmt.Sprintf(`
issuer: https://%[5]s

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
  # TODO: What should this be?
  issuer: https://%[5]s

  storage:
    type: kubernetes
    config:
      inCluster: true

  web:
    http: %[5]s

  enablePasswordDB: true

  # TODO: Templetize and generate this
  staticPasswords:
    - email: "admin@example.com"
      hash: "$2a$10$2b2cU8CPhOTaGrs1HRQuAueS7JTT5ZHsHSzYiFPm1leZck7Mc8T4W"
      username: "admin"
      userID: "08a8684b-db88-4b73-90a9-3cd1661f5466"

  staticClients:
    - id: epinio
      secret: %[3]s
      name: 'Epinio'
      # Where the app will be running.
      redirectURIs:
      - '%[4]s'
`,
		issuer,
		DexDeploymentID+"."+domain,
		"123", // TODO: Generate and store the secret somewhere
		fmt.Sprintf("https://%s.%s", EpinioDeploymentID, domain),
		fmt.Sprintf("%s.%s", DexDeploymentID, domain))

	configPath, err := helpers.CreateTmpFile(config)
	if err != nil {
		return err
	}
	defer os.Remove(configPath)

	helmCmd := fmt.Sprintf("helm %[1]s %[2]s --values %[3]s --namespace %[2]s %[4]s",
		action, DexDeploymentID, configPath, tarPath)

	if out, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
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
