package deployments

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/duration"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Registry struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &Registry{}

const (
	RegistryDeploymentID = "epinio-registry"
	registryVersion      = "0.1.0"
	registryChartFile    = "container-registry-0.1.0.tgz"
)

var registryAuthMemo *auth.PasswordAuth

func RegistryInstallAuth() (*auth.PasswordAuth, error) {
	if registryAuthMemo == nil {
		auth, err := auth.RandomPasswordAuth()
		if err != nil {
			return nil, err
		}
		registryAuthMemo = auth
	}
	return registryAuthMemo, nil
}

func (k *Registry) ID() string {
	return RegistryDeploymentID
}

func (k *Registry) Backup(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k *Registry) Restore(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k Registry) Describe() string {
	return emoji.Sprintf(":cloud:Registry version: %s\n:clipboard:Registry chart: %s", registryVersion, registryChartFile)
}

// Delete removes Registry from kubernetes cluster
func (k Registry) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Registry...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, RegistryDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", RegistryDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Registry because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling Registry: " + err.Error())
	}

	message := "Removing helm release " + RegistryDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			helmCmd := fmt.Sprintf("helm uninstall '%s' --namespace '%s'", RegistryDeploymentID, RegistryDeploymentID)
			return helpers.RunProc(helmCmd, currentdir, k.Debug)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", RegistryDeploymentID)
		} else {
			return errors.Wrapf(err, "Failed uninstalling helm release %s: %s", RegistryDeploymentID, out)
		}
	}

	message = "Deleting Registry namespace " + RegistryDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ctx, RegistryDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", RegistryDeploymentID)
	}

	ui.Success().Msg("Registry removed")

	return nil
}

func (k Registry) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	// Generate random credentials
	registryAuth, err := RegistryInstallAuth()
	if err != nil {
		return err
	}

	action := "install"
	if upgrade {
		action = "upgrade"
	}

	if err := c.CreateNamespace(ctx, RegistryDeploymentID, map[string]string{
		"quarks.cloudfoundry.org/monitored": "quarks-secret",
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	waitForQuarks(c, ctx, ui, duration.ToQuarksDeploymentReady())

	tarPath, err := helpers.ExtractFile(registryChartFile)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tarPath + " - " + err.Error())
	}
	defer os.Remove(tarPath)

	htpasswd, err := registryAuth.Htpassword()
	if err != nil {
		return errors.Wrap(err, "Failed to hash credentials")
	}

	// (**) See also `deployments/tekton.go`, func `createClusterRegistryCredsSecret`.
	helmCmd := fmt.Sprintf("helm %s %s --set 'auth.htpasswd=%s' --namespace %s %s",
		action, RegistryDeploymentID,
		htpasswd,
		RegistryDeploymentID, tarPath)
	if out, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.New("Failed installing Registry: " + out)
	}

	if err := c.WaitUntilPodBySelectorExist(ctx, ui, RegistryDeploymentID, "app.kubernetes.io/name=container-registry",
		duration.ToPodReady()); err != nil {
		return errors.Wrap(err, "failed waiting Registry deployment to come up")
	}
	if err := c.WaitForPodBySelectorRunning(ctx, ui, RegistryDeploymentID, "app.kubernetes.io/name=container-registry",
		duration.ToPodReady()); err != nil {
		return errors.Wrap(err, "failed waiting Registry deployment to come up")
	}

	// Workaround for cert-manager webhook service not being immediately ready.
	// More here: https://cert-manager.io/v1.2-docs/concepts/webhook/#webhook-connection-problems-shortly-after-cert-manager-installation
	err = retry.Do(func() error {
		return auth.CreateCertificate(ctx, c.RestConfig, RegistryDeploymentID, RegistryDeploymentID, domain)
	},
		retry.RetryIf(func(err error) bool {
			return strings.Contains(err.Error(), "failed calling webhook") ||
				strings.Contains(err.Error(), "EOF")
		}),
		retry.OnRetry(func(n uint, err error) {
			ui.Note().Msgf("retrying to create the epinio cert using cert-manager")
		}),
		retry.Delay(5*time.Second),
		retry.Attempts(10),
	)
	if err != nil {
		return errors.Wrap(err, "failed trying to create the epinio API server cert")
	}

	ui.Success().Msg("Registry deployed")

	return nil
}

func (k Registry) GetVersion() string {
	return registryVersion
}

func (k Registry) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		RegistryDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + RegistryDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Registry...")

	err = k.apply(ctx, c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Registry) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		RegistryDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + RegistryDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Registry...")

	return k.apply(ctx, c, ui, options, true)
}
