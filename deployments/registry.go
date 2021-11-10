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
	"github.com/go-logr/logr"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Registry struct {
	Debug   bool
	Timeout time.Duration
	Log     logr.Logger
}

var _ kubernetes.Deployment = &Registry{}

const (
	RegistryDeploymentID = "epinio-registry"
	RegistryCertSecret   = "epinio-registry-tls"
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

func (k Registry) ID() string {
	return RegistryDeploymentID
}

func (k Registry) Describe() string {
	return emoji.Sprintf(":cloud:Registry version: %s\n:clipboard:Registry chart: %s", registryVersion, registryChartFile)
}

func (k Registry) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

func (k Registry) PostDeleteCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	err := c.WaitForNamespaceMissing(ctx, ui, RegistryDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	ui.Success().Msg("Registry removed")

	return nil
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
			return helpers.RunProc(currentdir, k.Debug,
				"helm", "uninstall", RegistryDeploymentID, "--namespace", RegistryDeploymentID)
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

	return nil
}

func (k Registry) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool, log logr.Logger) error {
	// Generate random credentials
	registryAuth, err := RegistryInstallAuth()
	if err != nil {
		return err
	}

	action := "install"
	if upgrade {
		action = "upgrade"
	}

	log.Info("creating namespace", "namespace", RegistryDeploymentID)
	if err := c.CreateNamespace(ctx, RegistryDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	log.Info("extracting chart file", "name", registryChartFile)
	tarPath, err := helpers.ExtractFile(registryChartFile)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tarPath + " - " + err.Error())
	}
	defer os.Remove(tarPath)

	log.Info("local transient tar archive", "name", tarPath)

	htpasswd, err := registryAuth.Htpassword()
	if err != nil {
		return errors.Wrap(err, "Failed to hash credentials")
	}

	log.Info("htpasswd from credentials", "htpasswd", htpasswd)

	domain, err := options.GetString("system_domain", TektonDeploymentID)
	if err != nil {
		return errors.Wrap(err, "Couldn't get system_domain option")
	}

	log.Info("system domain", "domain", domain)

	if err := k.createCertificate(ctx, c, options, ui, log); err != nil {
		return errors.Wrap(err, "creating Registry TLS certificate")
	}

	log.Info("assembling helm command")

	// (**) See also `deployments/tekton.go`, func `createClusterRegistryCredsSecret`.
	helmArgs := []string{
		action, RegistryDeploymentID,
		`--namespace`, RegistryDeploymentID,
		tarPath,
		`--set`, `auth.htpasswd=` + htpasswd,
		`--set`, fmt.Sprintf("domain=%s.%s", RegistryDeploymentID, domain),
		`--set`, fmt.Sprintf(`createNodePort=%v`, !options.GetBoolNG("force-kube-internal-registry-tls")),
	}

	log.Info("assembled helm command", "command", strings.Join(append([]string{`helm`}, helmArgs...), " "))
	log.Info("run helm command")

	if out, err := helpers.RunProc(currentdir, k.Debug, "helm", helmArgs...); err != nil {
		return errors.New("Failed installing Registry: " + out)
	}

	log.Info("completed helm command")
	log.Info("waiting for pods to exist")

	if err := c.WaitUntilPodBySelectorExist(ctx, ui, RegistryDeploymentID, "app.kubernetes.io/name=container-registry",
		duration.ToPodReady()); err != nil {
		return errors.Wrap(err, "failed waiting Registry deployment to come up")
	}

	log.Info("waiting for pods to run")

	if err := c.WaitForPodBySelectorRunning(ctx, ui, RegistryDeploymentID, "app.kubernetes.io/name=container-registry",
		duration.ToPodReady()); err != nil {
		return errors.Wrap(err, "failed waiting Registry deployment to come up")
	}

	ui.Success().Msg("Registry deployed")

	return nil
}

func (k Registry) GetVersion() string {
	return registryVersion
}

func (k Registry) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	if options.GetStringNG("external-registry-url") != "" {
		ui.Exclamation().Msg("External registry configuration detected. Epinio won't install a registry")
		return nil
	}

	log := k.Log.WithName("Deploy")
	log.Info("start")
	defer log.Info("return")

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		RegistryDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + RegistryDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Registry...")

	err = k.apply(ctx, c, ui, options, false, log)
	if err != nil {
		return err
	}

	return nil
}

func (k Registry) createCertificate(ctx context.Context, c *kubernetes.Cluster, options kubernetes.InstallationOptions, ui *termui.UI, log logr.Logger) error {
	domain, err := options.GetString("system_domain", TektonDeploymentID)
	if err != nil {
		return errors.Wrap(err, "Couldn't get system_domain option")
	}

	log.Info("create properly annotated secret")

	// We need the empty certificate secret with a specific annotation
	// for it to be copied into `tekton-staging` namespace
	// https://cert-manager.io/docs/faq/kubed/#syncing-arbitrary-secrets-across-namespaces-using-kubed
	// TODO: We won't need to create an empty secret as soon as this is resolved:
	// https://github.com/jetstack/cert-manager/issues/2576
	// https://github.com/jetstack/cert-manager/pull/3828
	emptySecret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RegistryCertSecret,
			Namespace: RegistryDeploymentID,
			Annotations: map[string]string{
				"kubed.appscode.com/sync": fmt.Sprintf("cert-manager-tls=%s", RegistryDeploymentID),
			},
		},
		Type: v1.SecretTypeTLS,
		Data: map[string][]byte{
			"ca.crt":  nil,
			"tls.crt": nil,
			"tls.key": nil,
		},
	}
	err = c.CreateSecret(ctx, RegistryDeploymentID, emptySecret)
	if err != nil {
		return err
	}

	// Wait for the cert manager to be present and active. It is required
	log.Info("waiting for cert manager to be present and active")

	issuer := options.GetStringNG("tls-issuer")
	if err := waitForCertManagerReady(ctx, ui, c, issuer); err != nil {
		return errors.Wrap(err, "waiting for cert manager to be ready")
	}

	log.Info("issue registry cert")

	// Workaround for cert-manager webhook service not being immediately ready.
	// More here: https://cert-manager.io/v1.2-docs/concepts/webhook/#webhook-connection-problems-shortly-after-cert-manager-installation
	cert := auth.CertParam{
		Namespace: RegistryDeploymentID,
		Name:      RegistryDeploymentID,
		Issuer:    issuer,
		Domain:    fmt.Sprintf("%s.%s", RegistryDeploymentID, domain),
		Labels:    map[string]string{},
	}
	err = retry.Do(func() error {
		return auth.CreateCertificate(ctx, c, cert, nil)
	},
		retry.RetryIf(func(err error) bool {
			return helpers.Retryable(err.Error())
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

	// Wait until the cert is there
	if _, err := c.WaitForSecret(ctx, RegistryDeploymentID, RegistryDeploymentID+"-tls", duration.ToSecretCopied()); err != nil {
		return errors.Wrap(err, "waiting for the Registry tls certificate to be created")
	}

	return nil
}

func (k Registry) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	log := k.Log.WithName("Upgrade")
	log.Info("start")
	defer log.Info("return")

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		RegistryDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + RegistryDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Registry...")

	return k.apply(ctx, c, ui, options, true, log)
}
