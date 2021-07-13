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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

type CertManager struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &CertManager{}

const (
	CertManagerDeploymentID = "cert-manager"
	certManagerVersion      = "1.2.0"
	certManagerChartFile    = "cert-manager-v1.2.0.tgz"
	SelfSignedIssuer        = "selfsigned-issuer"
	LetsencryptIssuer       = "letsencrypt-production"
)

func (cm *CertManager) ID() string {
	return CertManagerDeploymentID
}

func (cm *CertManager) Backup(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (cm *CertManager) Restore(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (cm CertManager) Describe() string {
	return emoji.Sprintf(":cloud:CertManager version: %s\n:clipboard:CertManager chart: %s", certManagerVersion, certManagerChartFile)
}

func (cm CertManager) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing CertManager...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, CertManagerDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", CertManagerDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping CertManager because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling CertManager: " + err.Error())
	}

	err = cm.DeleteClusterIssuer(ctx, c)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting clusterissuer letsencrypt-production")
	}

	message := "Removing helm release " + CertManagerDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			helmCmd := fmt.Sprintf("helm uninstall cert-manager --namespace %s", CertManagerDeploymentID)
			return helpers.RunProc(helmCmd, currentdir, cm.Debug)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", CertManagerDeploymentID)
		} else {
			return errors.Wrapf(err, "Failed uninstalling helm release %s: %s", CertManagerDeploymentID, out)
		}
	}

	for _, crd := range []string{
		"certificaterequests.cert-manager.io",
		"certificates.cert-manager.io",
		"challenges.acme.cert-manager.io",
		"clusterissuers.cert-manager.io",
		"orders.acme.cert-manager.io",
	} {
		out, err := helpers.Kubectl("delete crds --ignore-not-found=true " + crd)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Deleting cert-manager CRD failed:\n%s", out))
		}
	}

	for _, webhook := range []string{
		"cert-manager-webhook",
	} {
		out, err := helpers.Kubectl("delete validatingwebhookconfigurations --ignore-not-found=true " + webhook)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Deleting cert-manager validatingwebhook failed:\n%s", out))
		}
	}

	for _, webhook := range []string{
		"cert-manager-webhook",
	} {
		out, err := helpers.Kubectl("delete mutatingwebhookconfigurations --ignore-not-found=true " + webhook)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Deleting cert-manager mutatingwebhook failed:\n%s", out))
		}
	}

	message = "Deleting CertManager namespace " + CertManagerDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ctx, CertManagerDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", CertManagerDeploymentID)
	}

	err = c.WaitForNamespaceMissing(ctx, ui, CertManagerDeploymentID, cm.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	ui.Success().Msg("CertManager removed")

	return nil
}

func (cm CertManager) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := c.CreateNamespace(ctx, CertManagerDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{}); err != nil {
		return err
	}

	// Setup CertManager helm values
	var helmArgs []string

	tarPath, err := helpers.ExtractFile(certManagerChartFile)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tarPath + " - " + err.Error())
	}
	defer os.Remove(tarPath)

	helmArgs = append(helmArgs, `--set installCRDs=true`)
	helmArgs = append(helmArgs, `--set extraArgs[0]=' --enable-certificate-owner-ref=true'`)
	helmCmd := fmt.Sprintf("helm %s cert-manager --namespace %s %s %s", action, CertManagerDeploymentID, tarPath, strings.Join(helmArgs, " "))

	if out, err := helpers.RunProc(helmCmd, currentdir, cm.Debug); err != nil {
		return errors.New("Failed installing CertManager: " + out)
	}

	for _, podname := range []string{
		"webhook",
		"cert-manager",
		"cainjector",
	} {
		if err := c.WaitUntilPodBySelectorExist(ctx, ui, CertManagerDeploymentID, "app.kubernetes.io/name="+podname, cm.Timeout); err != nil {
			return errors.Wrap(err, "failed waiting CertManager "+podname+" deployment to exist")
		}
		if err := c.WaitForPodBySelectorRunning(ctx, ui, CertManagerDeploymentID, "app.kubernetes.io/name="+podname, cm.Timeout); err != nil {
			return errors.Wrap(err, "failed waiting CertManager "+podname+" deployment to come up")
		}
	}

	emailAddress, err := options.GetOpt("email_address", "")
	if err != nil {
		return err
	}

	err = helpers.RunToSuccessWithTimeout(
		func() error {
			return cm.CreateClusterIssuer(ctx, c, fmt.Sprintf(clusterIssuerLetsencrypt, emailAddress.Value))
		}, duration.ToDeployment(), duration.PollInterval())
	if err != nil {
		if strings.Contains(err.Error(), "Timed out after") {
			return errors.Wrapf(err, "failed to create clusterissuer letsencrypt-production")
		}
		return err
	}

	err = helpers.RunToSuccessWithTimeout(
		func() error {
			return cm.CreateClusterIssuer(ctx, c, clusterIssuerLocal)
		}, duration.ToDeployment(), duration.PollInterval())
	if err != nil {
		if strings.Contains(err.Error(), "Timed out after") {
			return errors.Wrapf(err, "failed to create clusterissuer selfsigned-issuer")
		}
		return err
	}

	ui.Success().Msg("CertManager deployed")

	return nil
}

func (cm CertManager) GetVersion() string {
	return certManagerVersion
}

const clusterIssuerLetsencrypt = `{
	"apiVersion": "cert-manager.io/v1alpha2",
	"kind": "ClusterIssuer",
	"metadata": {
		"name": "letsencrypt-production"
	},
	"spec": {
		"acme" : {
			"email" : "%s",
			"server" : "https://acme-v02.api.letsencrypt.org/directory",
			"privateKeySecretRef" : {
				"name" : "letsencrypt-production"
			},
			"solvers" : [
			{
				"http01" : {
					"ingress" : {
						"class" : "traefik"
					}
				}
			}
			]
		}
	}
}`

const clusterIssuerLocal = `{
	"apiVersion": "cert-manager.io/v1alpha2",
	"kind": "ClusterIssuer",
	"metadata": {
		"name": "selfsigned-issuer"
	},
	"spec": {
		"selfSigned" : {}
	}
}`

func (cm CertManager) CreateClusterIssuer(ctx context.Context, c *kubernetes.Cluster, data string) error {
	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err := decoderUnstructured.Decode([]byte(data), nil, obj)
	if err != nil {
		return err
	}

	client, err := c.ClientCertManager()
	if err != nil {
		return err
	}

	_, err = client.Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (cm CertManager) DeleteClusterIssuer(ctx context.Context, c *kubernetes.Cluster) error {
	client, err := c.ClientCertManager()
	if err != nil {
		return err
	}

	err = client.Delete(ctx, "letsencrypt-production", metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	err = c.Kubectl.CoreV1().Secrets(CertManagerDeploymentID).Delete(ctx, "letsencrypt-production", metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (cm CertManager) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		CertManagerDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + CertManagerDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying CertManager...")

	err = cm.apply(ctx, c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (cm CertManager) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		CertManagerDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + CertManagerDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading CertManager...")

	return cm.apply(ctx, c, ui, options, true)
}

func waitForCertManagerReady(ctx context.Context, ui *termui.UI, c *kubernetes.Cluster) error {
	for _, deployment := range []string{
		"cert-manager",
		"cert-manager-webhook",
		"cert-manager-cainjector",
	} {

		if err := c.WaitUntilDeploymentExists(ctx, ui, CertManagerDeploymentID, deployment, duration.ToCertManagerReady()); err != nil {
			return errors.Wrapf(err, "failed waiting CertManager %s deployment to exist in namespace %s", deployment, CertManagerDeploymentID)
		}

		if err := c.WaitForDeploymentCompleted(ctx, ui, CertManagerDeploymentID, deployment, duration.ToCertManagerReady()); err != nil {
			return errors.Wrapf(err, "failed waiting CertManager %s deployment to be ready in namespace %s", deployment, CertManagerDeploymentID)
		}
	}

	return nil
}
