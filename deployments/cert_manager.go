package deployments

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/kubernetes"
	"github.com/epinio/epinio/termui"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
)

type CertManager struct {
	Debug   bool
	Timeout time.Duration
}

const (
	CertManagerDeploymentID = "cert-manager"
	certManagerVersion      = "1.2.0"
	certManagerChartURL     = "https://charts.jetstack.io/charts/cert-manager-v1.2.0.tgz"
)

func (cm *CertManager) ID() string {
	return CertManagerDeploymentID
}

func (cm *CertManager) Backup(c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (cm *CertManager) Restore(c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (cm CertManager) Describe() string {
	return emoji.Sprintf(":cloud:CertManager version: %s\n:clipboard:CertManager chart: %s", certManagerVersion, certManagerChartURL)
}

func (cm CertManager) Delete(c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing CertManager...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(CertManagerDeploymentID)
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

	err = cm.DeleteClusterIssuer(c)
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
			return "", c.DeleteNamespace(CertManagerDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", CertManagerDeploymentID)
	}

	ui.Success().Msg("CertManager removed")

	return nil
}

func (cm CertManager) apply(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Setup CertManager helm values
	var helmArgs []string

	helmArgs = append(helmArgs, `--set installCRDs=true`)
	helmCmd := fmt.Sprintf("helm %s cert-manager --create-namespace --namespace %s %s %s", action, CertManagerDeploymentID, certManagerChartURL, strings.Join(helmArgs, " "))

	if out, err := helpers.RunProc(helmCmd, currentdir, cm.Debug); err != nil {
		return errors.New("Failed installing CertManager: " + out)
	}

	for _, podname := range []string{
		"webhook",
		"cert-manager",
		"cainjector",
	} {
		if err := c.WaitUntilPodBySelectorExist(ui, CertManagerDeploymentID, "app.kubernetes.io/name="+podname, cm.Timeout); err != nil {
			return errors.Wrap(err, "failed waiting CertManager "+podname+" deployment to exist")
		}
		if err := c.WaitForPodBySelectorRunning(ui, CertManagerDeploymentID, "app.kubernetes.io/name="+podname, cm.Timeout); err != nil {
			return errors.Wrap(err, "failed waiting CertManager "+podname+" deployment to come up")
		}
	}

	err = c.LabelNamespace(CertManagerDeploymentID, kubernetes.EpinioDeploymentLabelKey, kubernetes.EpinioDeploymentLabelValue)
	if err != nil {
		return err
	}

	emailAddress, err := options.GetOpt("email_address", "")
	if err != nil {
		return err
	}

	err = helpers.RunToSuccessWithTimeout(
		func() error {
			return cm.CreateClusterIssuer(c, emailAddress.Value.(string))
		}, duration.ToDeployment(), duration.PollInterval())
	if err != nil {
		if strings.Contains(err.Error(), "Timed out after") {
			return errors.Wrapf(err, "failed to create clusterissuer letsencrypt-production")
		}
		return err
	}

	ui.Success().Msg("CertManager deployed")

	return nil
}

func (cm CertManager) GetVersion() string {
	return certManagerVersion
}

func (cm CertManager) CreateClusterIssuer(c *kubernetes.Cluster, emailAddress string) error {
	data := fmt.Sprintf(`{
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
	}`, emailAddress)

	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err := decoderUnstructured.Decode([]byte(data), nil, obj)
	if err != nil {
		return err
	}

	clusterIssuerGVR := schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1alpha2",
		Resource: "clusterissuers",
	}

	dynamicClient, err := dynamic.NewForConfig(c.RestConfig)
	if err != nil {
		return err
	}

	_, err = dynamicClient.Resource(clusterIssuerGVR).
		Create(context.Background(),
			obj,
			metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (cm CertManager) DeleteClusterIssuer(c *kubernetes.Cluster) error {
	clusterIssuerGVR := schema.GroupVersionResource{
		Group:    "cert-manager.io",
		Version:  "v1alpha2",
		Resource: "clusterissuers",
	}

	dynamicClient, err := dynamic.NewForConfig(c.RestConfig)
	if err != nil {
		return err
	}

	err = dynamicClient.Resource(clusterIssuerGVR).
		Delete(context.Background(),
			"letsencrypt-production",
			metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	err = c.Kubectl.CoreV1().Secrets(CertManagerDeploymentID).Delete(context.Background(), "letsencrypt-production", metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (cm CertManager) Deploy(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		CertManagerDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + CertManagerDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying CertManager...")

	err = cm.apply(c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (cm CertManager) Upgrade(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		CertManagerDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + CertManagerDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading CertManager...")

	return cm.apply(c, ui, options, true)
}
