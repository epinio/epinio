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

type Traefik struct {
	Debug   bool
	Timeout int
}

const (
	TraefikDeploymentID = "traefik"
	traefikVersion      = "9.11.0"
	traefikChartURL     = "https://helm.traefik.io/traefik/traefik-9.11.0.tgz"
)

func (k *Traefik) ID() string {
	return TraefikDeploymentID
}

func (k *Traefik) Backup(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k *Traefik) Restore(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k Traefik) Describe() string {
	return emoji.Sprintf(":cloud:Traefik version: %s\n:clipboard:Traefik Ingress chart: %s", traefikVersion, traefikChartURL)
}

// Delete removes traefik from kubernetes cluster
func (k Traefik) Delete(c *kubernetes.Cluster, ui *ui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Traefik...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(TraefikDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", TraefikDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Traefik because namespace either doesn't exist or not owned by Carrier")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling Traefik: " + err.Error())
	}

	message := "Removing helm release " + TraefikDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			helmCmd := fmt.Sprintf("helm uninstall traefik --namespace '%s'", TraefikDeploymentID)
			return helpers.RunProc(helmCmd, currentdir, k.Debug)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", TraefikDeploymentID)
		} else {
			return errors.Wrapf(err, "Failed uninstalling helm release %s: %s", TraefikDeploymentID, out)
		}
	}

	message = "Deleting Traefik namespace " + TraefikDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(TraefikDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", TraefikDeploymentID)
	}

	ui.Success().Msg("Traefik removed")

	return nil
}

func (k Traefik) apply(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Setup Traefik helm values
	var helmArgs []string

	// Disable sending anonymous usage statistics
	// https://github.com/traefik/traefik-helm-chart/blob/v9.11.0/traefik/values.yaml#L170
	// Overwrite globalArguments until https://github.com/traefik/traefik-helm-chart/issues/357 is fixed
	helmArgs = append(helmArgs, `--set "globalArguments="`)

	helmCmd := fmt.Sprintf("helm %s traefik --create-namespace --namespace %s %s %s", action, TraefikDeploymentID, traefikChartURL, strings.Join(helmArgs, " "))
	if out, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed installing Traefik: %s\n", out))
	}

	err = c.LabelNamespace(TraefikDeploymentID, kubernetes.CarrierDeploymentLabelKey, kubernetes.CarrierDeploymentLabelValue)
	if err != nil {
		return err
	}

	if err := c.WaitUntilPodBySelectorExist(ui, TraefikDeploymentID, "app.kubernetes.io/name=traefik", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Traefik Ingress deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning(ui, TraefikDeploymentID, "app.kubernetes.io/name=traefik", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Traefik Ingress deployment to come up")
	}

	ui.Success().Msg("Traefik Ingress deployed")

	return nil
}

func (k Traefik) GetVersion() string {
	return traefikVersion
}

func (k Traefik) Deploy(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		TraefikDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + TraefikDeploymentID + " present already")
	}

	_, err = c.Kubectl.CoreV1().Services("kube-system").Get(
		context.Background(),
		"traefik",
		metav1.GetOptions{},
	)
	if err == nil {
		ui.Exclamation().Msg("Traefik Ingress already installed, skipping")

		return nil
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Traefik Ingress...")

	return k.apply(c, ui, options, false)
}

func (k Traefik) Upgrade(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		TraefikDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + TraefikDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Traefik Ingress...")

	return k.apply(c, ui, options, true)
}
