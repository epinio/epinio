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
	"github.com/epinio/epinio/internal/duration"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Kubed struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &Kubed{}

const (
	KubedDeploymentID = "kubed"
	KubedVersion      = "v0.12.0"
	KubedChartFile    = "kubed-v0.12.0.tgz"
)

func (k Kubed) ID() string {
	return KubedDeploymentID
}

func (k Kubed) Describe() string {
	return emoji.Sprintf(":cloud:Epinio version: %s\n", KubedVersion)

}

func (k Kubed) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

func (k Kubed) PostDeleteCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	err := c.WaitForNamespaceMissing(ctx, ui, KubedDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	ui.Success().Msg("Kubed removed")

	return nil
}

// Delete removes Kubed from kubernetes cluster
func (k Kubed) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Kubed...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, KubedDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not",
			KubedDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Kubed because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling Kubed: " + err.Error())
	}

	message := "Removing helm release " + KubedDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.RunProc(currentdir, k.Debug,
				"helm", "uninstall", "kubed", "--namespace", KubedDeploymentID)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", KubedDeploymentID)
		} else {
			return errors.New("Failed uninstalling Kubed: " + out)
		}
	}

	message = "Deleting Kubed namespace " + KubedDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ctx, KubedDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", KubedDeploymentID)
	}

	return nil
}

func (k Kubed) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	if err := c.CreateNamespace(ctx, KubedDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{}); err != nil {
		return err
	}

	currentdir, _ := os.Getwd()
	tarPath, err := helpers.ExtractFile(KubedChartFile)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tarPath + " - " + err.Error())
	}
	defer os.Remove(tarPath)

	// Setup Kubed helm values
	if out, err := helpers.RunProc(currentdir, k.Debug,
		"helm", action, "kubed", "--namespace", KubedDeploymentID, tarPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed installing Kubed, Returning\n%s", out))
	}

	if err := c.WaitForDeploymentCompleted(ctx, ui, KubedDeploymentID, "kubed", duration.ToKubedReady()); err != nil {
		return errors.Wrapf(err, "failed waiting kubed deployment to be ready in namespace %s", KubedDeploymentID)
	}

	ui.Success().Msg("Kubed deployed")

	return nil
}

func (k Kubed) GetVersion() string {
	return KubedVersion
}

func (k Kubed) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		KubedDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + KubedDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Kubed...")

	return k.apply(ctx, c, ui, options, false)
}

func (k Kubed) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		KubedDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + KubedDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Kubed...")

	return k.apply(ctx, c, ui, options, true)
}
