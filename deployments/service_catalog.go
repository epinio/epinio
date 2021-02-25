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

type ServiceCatalog struct {
	Debug   bool
	Timeout int
}

const (
	ServiceCatalogDeploymentID = "service-catalog"
	serviceCatalogVersion      = "0.3.1"
	serviceCatalogChartFile    = "catalog-0.3.1.tgz"
)

func (k *ServiceCatalog) ID() string {
	return ServiceCatalogDeploymentID
}

func (k *ServiceCatalog) Backup(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k *ServiceCatalog) Restore(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k ServiceCatalog) Describe() string {
	return emoji.Sprintf(":cloud:Service Catalog version: %s\n", serviceCatalogVersion)
}

// Delete removes ServiceCatalog from kubernetes cluster
func (k ServiceCatalog) Delete(c *kubernetes.Cluster, ui *ui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing ServiceCatalog...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ServiceCatalogDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", ServiceCatalogDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping ServiceCatalog because namespace either doesn't exist or not owned by Carrier")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling ServiceCatalog: " + err.Error())
	}

	message := "Removing helm release " + ServiceCatalogDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			helmCmd := fmt.Sprintf("helm uninstall '%s' --namespace %s", ServiceCatalogDeploymentID, ServiceCatalogDeploymentID)
			return helpers.RunProc(helmCmd, currentdir, k.Debug)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", ServiceCatalogDeploymentID)
		} else {
			return errors.Wrapf(err, "Failed uninstalling helm release %s: %s", ServiceCatalogDeploymentID, out)
		}
	}

	message = "Deleting ServiceCatalog namespace " + ServiceCatalogDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ServiceCatalogDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", ServiceCatalogDeploymentID)
	}

	ui.Success().Msg("ServiceCatalog removed")

	return nil
}

func (k ServiceCatalog) apply(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	tarPath, err := helpers.ExtractFile(serviceCatalogChartFile)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tarPath + " - " + err.Error())
	}
	defer os.Remove(tarPath)

	helmCmd := fmt.Sprintf("helm %s %s --create-namespace --namespace %s %s", action, ServiceCatalogDeploymentID, ServiceCatalogDeploymentID, tarPath)
	if out, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.New("Failed installing ServiceCatalog: " + out)
	}

	err = c.LabelNamespace(ServiceCatalogDeploymentID, kubernetes.CarrierDeploymentLabelKey, kubernetes.CarrierDeploymentLabelValue)
	if err != nil {
		return err
	}
	if err := c.WaitUntilPodBySelectorExist(ui, ServiceCatalogDeploymentID, "app=service-catalog-catalog-controller-manager", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting ServiceCatalog controller manager to come up")
	}
	if err := c.WaitForPodBySelectorRunning(ui, ServiceCatalogDeploymentID, "app=service-catalog-catalog-controller-manager", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting ServiceCatalog controller manager to come be running")
	}
	if err := c.WaitUntilPodBySelectorExist(ui, ServiceCatalogDeploymentID, "app=service-catalog-catalog-webhook", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting ServiceCatalog webhook to come up")
	}
	if err := c.WaitForPodBySelectorRunning(ui, ServiceCatalogDeploymentID, "app=service-catalog-catalog-webhook", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting ServiceCatalog webhook to come be running")
	}

	ui.Success().Msg("ServiceCatalog deployed")

	return nil
}

func (k ServiceCatalog) GetVersion() string {
	return serviceCatalogVersion
}

func (k ServiceCatalog) Deploy(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		ServiceCatalogDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + ServiceCatalogDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying ServiceCatalog...")

	err = k.apply(c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k ServiceCatalog) Upgrade(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		ServiceCatalogDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + ServiceCatalogDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading ServiceCatalog...")

	return k.apply(c, ui, options, true)
}
