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

type Quarks struct {
	Debug   bool
	Timeout int
}

const (
	QuarksDeploymentID = "quarks"
	quarksVersion      = "6.1.17+0.gec409fd7"
	quarksChartURL     = "https://cloudfoundry-incubator.github.io/quarks-helm/quarks-secret-0.0.755-ge100fdc.tgz"
)

func (k *Quarks) ID() string {
	return QuarksDeploymentID
}

func (k *Quarks) Backup(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k *Quarks) Restore(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k Quarks) Describe() string {
	return emoji.Sprintf(":cloud:Quarks version: %s\n:clipboard:Quarks chart: %s", quarksVersion, quarksChartURL)
}

// Delete removes Quarks from kubernetes cluster
func (k Quarks) Delete(c *kubernetes.Cluster, ui *ui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Quarks...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(QuarksDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", QuarksDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Quarks because namespace either doesn't exist or not owned by Carrier")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling Quarks: " + err.Error())
	}

	message := "Removing helm release " + QuarksDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			helmCmd := fmt.Sprintf("helm uninstall quarks --namespace %s", QuarksDeploymentID)
			return helpers.RunProc(helmCmd, currentdir, k.Debug)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", QuarksDeploymentID)
		} else {
			return errors.New("Failed uninstalling Quarks: " + out)
		}
	}

	for _, crd := range []string{
		"quarkssecrets.quarks.cloudfoundry.org",
	} {
		out, err := helpers.Kubectl("delete crds --ignore-not-found=true " + crd)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Deleting quarks CRD failed:\n%s", out))
		}
	}

	message = "Deleting Quarks namespace " + QuarksDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(QuarksDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", QuarksDeploymentID)
	}

	ui.Success().Msg("Quarks removed")

	return nil
}

func (k Quarks) apply(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	currentdir, _ := os.Getwd()

	// Setup Quarks helm values
	var helmArgs []string

	helmArgs = append(helmArgs, "--set global.monitoredID=quarks-secret")

	helmCmd := fmt.Sprintf("helm %s quarks --create-namespace --namespace %s %s %s", action, QuarksDeploymentID, quarksChartURL, strings.Join(helmArgs, " "))
	if _, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.New("Failed installing Quarks")
	}

	if err := c.WaitUntilPodBySelectorExist(ui, QuarksDeploymentID, "name=quarks-secret", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Quarks quarks-secret deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning(ui, QuarksDeploymentID, "name=quarks-secret", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Quarks quarks-secret deployment to come up")
	}

	err := c.LabelNamespace(QuarksDeploymentID, kubernetes.CarrierDeploymentLabelKey, kubernetes.CarrierDeploymentLabelValue)
	if err != nil {
		return err
	}

	ui.Success().Msg("Quarks deployed")

	return nil
}

func (k Quarks) GetVersion() string {
	return quarksVersion
}

func (k Quarks) Deploy(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		QuarksDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + QuarksDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Quarks...")

	return k.apply(c, ui, options, false)
}

func (k Quarks) Upgrade(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		QuarksDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + QuarksDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Quarks...")

	return k.apply(c, ui, options, true)
}
