package deployments

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"github.com/suse/carrier/cli/helpers"
	"github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/paas/ui"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GoogleServices struct {
	Debug   bool
	Timeout int
}

const (
	GoogleServicesDeploymentID = "google-service-broker"
	googleServicesVersion      = "0.1.0"
	googleServicesChartFile    = "gcp-service-broker-0.1.0.tgz"
)

func (k *GoogleServices) ID() string {
	return GoogleServicesDeploymentID
}

func (k *GoogleServices) Backup(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k *GoogleServices) Restore(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k GoogleServices) Describe() string {
	return emoji.Sprintf(":cloud:GoogleServices version: %s\n", googleServicesVersion)
}

// Delete removes GoogleServices from kubernetes cluster
func (k GoogleServices) Delete(c *kubernetes.Cluster, ui *ui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing GoogleServices...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(GoogleServicesDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", GoogleServicesDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping GoogleServices because namespace either doesn't exist or not owned by Carrier")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling GoogleServices: " + err.Error())
	}

	message := "Removing helm release " + GoogleServicesDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			helmCmd := fmt.Sprintf("helm uninstall '%s' --namespace %s", GoogleServicesDeploymentID, GoogleServicesDeploymentID)
			return helpers.RunProc(helmCmd, currentdir, k.Debug)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", GoogleServicesDeploymentID)
		} else {
			return errors.Wrapf(err, "Failed uninstalling helm release %s: %s", GoogleServicesDeploymentID, out)
		}
	}

	message = "Deleting GoogleServices namespace " + GoogleServicesDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(GoogleServicesDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", GoogleServicesDeploymentID)
	}

	ui.Success().Msg("GoogleServices removed")

	return nil
}

func (k GoogleServices) apply(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	tarPath, err := helpers.ExtractFile(googleServicesChartFile)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tarPath + " - " + err.Error())
	}
	defer os.Remove(tarPath)

	helmCmd := fmt.Sprintf("helm %s %s --create-namespace --namespace %s %s", action, GoogleServicesDeploymentID, GoogleServicesDeploymentID, tarPath)
	if out, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.New("Failed installing GoogleServices: " + out)
	}

	err = c.LabelNamespace(GoogleServicesDeploymentID, kubernetes.CarrierDeploymentLabelKey, kubernetes.CarrierDeploymentLabelValue)
	if err != nil {
		return err
	}
	if err := c.WaitUntilPodBySelectorExist(ui, GoogleServicesDeploymentID, "app=minibroker-minibroker", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting GoogleServices to come up")
	}
	if err := c.WaitForPodBySelectorRunning(ui, GoogleServicesDeploymentID, "app=minibroker-minibroker", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting GoogleServices to come be running")
	}

	ui.Success().Msg("GoogleServices deployed")

	return nil
}

func (k GoogleServices) GetVersion() string {
	return googleServicesVersion
}

func (k GoogleServices) Deploy(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		GoogleServicesDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + GoogleServicesDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying GoogleServices...")

	err = k.apply(c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k GoogleServices) Upgrade(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		GoogleServicesDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + GoogleServicesDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading GoogleServices...")

	return k.apply(c, ui, options, true)
}
