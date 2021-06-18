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

type Quarks struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &Quarks{}

const (
	QuarksDeploymentID = "quarks"
	quarksVersion      = "1.0.760"
	quarksChartFile    = "quarks-secret-1.0.760.tgz"
)

var (
	quarksLiteImageTag = fmt.Sprintf("v%s-lite", quarksVersion) // Use the "lite" version of the image.
)

func (k *Quarks) ID() string {
	return QuarksDeploymentID
}

func (k *Quarks) Backup(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k *Quarks) Restore(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k Quarks) Describe() string {
	return emoji.Sprintf(":cloud:Quarks version: %s\n:clipboard:Quarks chart: %s",
		quarksVersion, quarksChartFile)
}

// Delete removes Quarks from kubernetes cluster
func (k Quarks) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Quarks...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, QuarksDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not",
			QuarksDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Quarks because namespace either doesn't exist or not owned by Epinio")
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
			return "", c.DeleteNamespace(ctx, QuarksDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", QuarksDeploymentID)
	}

	ui.Success().Msg("Quarks removed")

	return nil
}

func (k Quarks) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	if err := c.CreateNamespace(ctx, QuarksDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{}); err != nil {
		return err
	}

	currentdir, _ := os.Getwd()

	tarPath, err := helpers.ExtractFile(quarksChartFile)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tarPath + " - " + err.Error())
	}
	defer os.Remove(tarPath)

	// Setup Quarks helm values
	var helmArgs = []string{
		"--set logLevel=info",
		"--set image.tag=" + quarksLiteImageTag,
	}

	helmArgs = append(helmArgs, "--set global.monitoredID=quarks-secret")

	helmCmd := fmt.Sprintf("helm %s quarks --namespace %s %s %s", action, QuarksDeploymentID, tarPath, strings.Join(helmArgs, " "))
	if out, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed installing Quarks:\n%s\nReturning\n%s", helmCmd, out))
	}

	waitForQuarks(c, ctx, ui, duration.ToQuarksDeploymentReady())

	ui.Success().Msg("Quarks deployed")

	return nil
}

func (k Quarks) GetVersion() string {
	return quarksVersion
}

func (k Quarks) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		QuarksDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + QuarksDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Quarks...")

	return k.apply(ctx, c, ui, options, false)
}

func (k Quarks) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		QuarksDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + QuarksDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Quarks...")

	return k.apply(ctx, c, ui, options, true)
}

// This method can be used by any other deployment that needs to wait until Quarks
// Deployment is ready.
func waitForQuarks(c *kubernetes.Cluster, ctx context.Context, ui *termui.UI, timeout time.Duration) error {
	if err := c.WaitUntilDeploymentExists(ctx, ui, QuarksDeploymentID, "quarks-secret", duration.ToQuarksDeploymentReady()); err != nil {
		return errors.Wrap(err, "failed waiting Quarks quarks-secret deployment to exist")
	}
	if err := c.WaitForDeploymentCompleted(ctx, ui, QuarksDeploymentID, "quarks-secret", duration.ToQuarksDeploymentReady()); err != nil {
		return errors.Wrap(err, "failed waiting Quarks quarks-secret deployment to exist")
	}

	message := "Waiting for QuarksSecret CRD to be established"
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.Kubectl(fmt.Sprintf("wait --for=condition=established --timeout=%ds crd/quarkssecrets.quarks.cloudfoundry.org", int(timeout/time.Second)))
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	return nil
}
