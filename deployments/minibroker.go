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

type Minibroker struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &Minibroker{}

const (
	MinibrokerDeploymentID = "minibroker"
	minibrokerVersion      = "1.2.0"
	minibrokerChartFile    = "minibroker-1.2.0.tgz"
)

func (k Minibroker) ID() string {
	return MinibrokerDeploymentID
}

func (k Minibroker) Describe() string {
	return emoji.Sprintf(":cloud:Minibroker version: %s\n", minibrokerVersion)
}

func (k *Minibroker) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

// Delete removes Minibroker from kubernetes cluster
func (k Minibroker) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Minibroker...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, MinibrokerDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", MinibrokerDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Minibroker because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling Minibroker: " + err.Error())
	}

	message := "Removing helm release " + MinibrokerDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.RunProc(currentdir, k.Debug,
				"helm", "uninstall", MinibrokerDeploymentID, "--namespace", MinibrokerDeploymentID)
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
			return "", c.DeleteNamespace(ctx, MinibrokerDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", MinibrokerDeploymentID)
	}

	err = c.WaitForNamespaceMissing(ctx, ui, MinibrokerDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	ui.Success().Msg("Minibroker removed")

	return nil
}

func (k Minibroker) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	if err := c.CreateNamespace(ctx, MinibrokerDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
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

	if out, err := helpers.RunProc(currentdir, k.Debug,
		"helm", action, MinibrokerDeploymentID, "--namespace", MinibrokerDeploymentID, tarPath); err != nil {
		return errors.New("Failed installing Minibroker: " + out)
	}

	if err := c.WaitUntilPodBySelectorExist(ctx, ui, MinibrokerDeploymentID, "app=minibroker-minibroker", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Minibroker to come up")
	}
	if err := c.WaitForPodBySelectorRunning(ctx, ui, MinibrokerDeploymentID, "app=minibroker-minibroker", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Minibroker to come be running")
	}

	ui.Success().Msg("Minibroker deployed")

	return nil
}

func (k Minibroker) GetVersion() string {
	return minibrokerVersion
}

func (k Minibroker) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, MinibrokerDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", MinibrokerDeploymentID)
	}
	if existsAndOwned {
		ui.Exclamation().Msg("Minibroker already installed, skipping")
		return nil
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Minibroker...")

	err = k.apply(ctx, c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Minibroker) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		MinibrokerDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + MinibrokerDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Minibroker...")

	return k.apply(ctx, c, ui, options, true)
}
