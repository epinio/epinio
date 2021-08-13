package deployments

import (
	"context"
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

type ServiceCatalog struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &ServiceCatalog{}

const (
	ServiceCatalogDeploymentID = "service-catalog"
	serviceCatalogVersion      = "0.3.1"
	serviceCatalogChartFile    = "catalog-0.3.1.tgz"
)

func (k ServiceCatalog) ID() string {
	return ServiceCatalogDeploymentID
}

func (k ServiceCatalog) Describe() string {
	return emoji.Sprintf(":cloud:Service Catalog version: %s\n", serviceCatalogVersion)
}

func (k ServiceCatalog) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

func (k ServiceCatalog) PostDeleteCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	err := c.WaitForNamespaceMissing(ctx, ui, ServiceCatalogDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	ui.Success().Msg("ServiceCatalog removed")

	return nil
}

// Delete removes ServiceCatalog from kubernetes cluster
func (k ServiceCatalog) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing ServiceCatalog...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, ServiceCatalogDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", ServiceCatalogDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping ServiceCatalog because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling ServiceCatalog: " + err.Error())
	}

	message := "Removing helm release " + ServiceCatalogDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.RunProc(currentdir, k.Debug,
				"helm", "uninstall", ServiceCatalogDeploymentID, "--namespace", ServiceCatalogDeploymentID)
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
			return "", c.DeleteNamespace(ctx, ServiceCatalogDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", ServiceCatalogDeploymentID)
	}

	return nil
}

func (k ServiceCatalog) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	if err := c.CreateNamespace(ctx, ServiceCatalogDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{}); err != nil {
		return err
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

	if out, err := helpers.RunProc(currentdir, k.Debug,
		"helm", action, ServiceCatalogDeploymentID, "--namespace", ServiceCatalogDeploymentID, tarPath); err != nil {
		return errors.New("Failed installing ServiceCatalog: " + out)
	}

	if err := c.WaitUntilPodBySelectorExist(ctx, ui, ServiceCatalogDeploymentID, "app=service-catalog-catalog-controller-manager", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting ServiceCatalog controller manager to come up")
	}
	if err := c.WaitForPodBySelectorRunning(ctx, ui, ServiceCatalogDeploymentID, "app=service-catalog-catalog-controller-manager", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting ServiceCatalog controller manager to come be running")
	}
	if err := c.WaitUntilPodBySelectorExist(ctx, ui, ServiceCatalogDeploymentID, "app=service-catalog-catalog-webhook", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting ServiceCatalog webhook to come up")
	}
	if err := c.WaitForPodBySelectorRunning(ctx, ui, ServiceCatalogDeploymentID, "app=service-catalog-catalog-webhook", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting ServiceCatalog webhook to come be running")
	}
	if err := c.WaitForPodBySelectorRunning(ctx, ui, ServiceCatalogDeploymentID, "app=service-catalog-catalog-webhook", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting ServiceCatalog webhook to come be running")
	}
	if err = c.WaitForCRD(ctx, ui, "clusterservicebrokers.servicecatalog.k8s.io", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting for CRD clusterservicebrokers.servicecatalog.k8s.io to become available")
	}

	ui.Success().Msg("ServiceCatalog deployed")

	return nil
}

func (k ServiceCatalog) GetVersion() string {
	return serviceCatalogVersion
}

func (k ServiceCatalog) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		ServiceCatalogDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + ServiceCatalogDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying ServiceCatalog...")

	err = k.apply(ctx, c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k ServiceCatalog) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		ServiceCatalogDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + ServiceCatalogDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading ServiceCatalog...")

	return k.apply(ctx, c, ui, options, true)
}
