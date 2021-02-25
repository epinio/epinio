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

type Minibroker struct {
	Debug   bool
	Timeout int
}

const (
	MinibrokerDeploymentID = "minibroker"
	minibrokerVersion      = "1.2.0"
	minibrokerChartFile    = "minibroker-1.2.0.tgz"
)

func (k *Minibroker) ID() string {
	return MinibrokerDeploymentID
}

func (k *Minibroker) Backup(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k *Minibroker) Restore(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k Minibroker) Describe() string {
	return emoji.Sprintf(":cloud:Minibroker version: %s\n", minibrokerVersion)
}

// Delete removes Minibroker from kubernetes cluster
func (k Minibroker) Delete(c *kubernetes.Cluster, ui *ui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Minibroker...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(MinibrokerDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", MinibrokerDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Minibroker because namespace either doesn't exist or not owned by Carrier")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling Minibroker: " + err.Error())
	}

	message := "Removing helm release " + MinibrokerDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			helmCmd := fmt.Sprintf("helm uninstall '%s' --namespace %s", MinibrokerDeploymentID, MinibrokerDeploymentID)
			return helpers.RunProc(helmCmd, currentdir, k.Debug)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", MinibrokerDeploymentID)
		} else {
			return errors.Wrapf(err, "Failed uninstalling helm release %s: %s", MinibrokerDeploymentID, out)
		}
	}

	message = "Deleting Minibroker namespace " + MinibrokerDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(MinibrokerDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", MinibrokerDeploymentID)
	}

	err = c.WaitForNamespaceMissing(ui, MinibrokerDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	ui.Success().Msg("Minibroker removed")

	return nil
}

func (k Minibroker) apply(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	tarPath, err := helpers.ExtractFile(minibrokerChartFile)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tarPath + " - " + err.Error())
	}
	defer os.Remove(tarPath)

	helmCmd := fmt.Sprintf("helm %s %s --create-namespace --namespace %s %s", action, MinibrokerDeploymentID, MinibrokerDeploymentID, tarPath)
	if out, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.New("Failed installing Minibroker: " + out)
	}

	err = c.LabelNamespace(MinibrokerDeploymentID, kubernetes.CarrierDeploymentLabelKey, kubernetes.CarrierDeploymentLabelValue)
	if err != nil {
		return err
	}
	if err := c.WaitUntilPodBySelectorExist(ui, MinibrokerDeploymentID, "app=minibroker-minibroker", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Minibroker to come up")
	}
	if err := c.WaitForPodBySelectorRunning(ui, MinibrokerDeploymentID, "app=minibroker-minibroker", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Minibroker to come be running")
	}

	ui.Success().Msg("Minibroker deployed")

	return nil
}

func (k Minibroker) GetVersion() string {
	return minibrokerVersion
}

func (k Minibroker) Deploy(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		MinibrokerDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + MinibrokerDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Minibroker...")

	err = k.apply(c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Minibroker) Upgrade(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		MinibrokerDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + MinibrokerDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Minibroker...")

	return k.apply(c, ui, options, true)
}
