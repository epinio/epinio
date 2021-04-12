package deployments

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/kubernetes"
	"github.com/epinio/epinio/termui"
	"github.com/epinio/epinio/version"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type Epinio struct {
	Debug   bool
	Timeout time.Duration
}

const (
	EpinioDeploymentID  = "epinio"
	epinioBinaryPVCYaml = "epinio/binary-pvc.yaml"
	epinioCopierPodYaml = "epinio/copier-pod.yaml"
	epinioServerYaml    = "epinio/server.yaml"
)

func (k *Epinio) ID() string {
	return EpinioDeploymentID
}

func (k *Epinio) Backup(c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k *Epinio) Restore(c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k Epinio) Describe() string {
	return emoji.Sprintf(":cloud:Epinio version: %s\n", version.Version)
}

// Delete removes Epinio from kubernetes cluster
func (k Epinio) Delete(c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Epinio...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(EpinioDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", EpinioDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Epinio because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	// We are taking a shortcut here. When we applied the file we had to replace
	// ##current_epinio_version## with the correct version. No need to do any parsing when
	// deleting though.
	if out, err := helpers.KubectlDeleteEmbeddedYaml(epinioServerYaml, true); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", epinioServerYaml, out))
	}

	message := "Deleting Epinio namespace " + EpinioDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(EpinioDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", EpinioDeploymentID)
	}

	ui.Success().Msg("Epinio removed")

	return nil
}

func (k Epinio) apply(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	if err := k.createEpinioNamespace(c, ui); err != nil {
		return err
	}

	if out, err := applyEpinioServerYaml(c, ui); err != nil {
		return errors.Wrap(err, out)
	}

	domain, err := options.GetString("system_domain", TektonDeploymentID)
	if err != nil {
		return errors.Wrap(err, "Couldn't get system_domain option")
	}

	message := "Creating Epinio server ingress"
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", k.createIngress(c, EpinioDeploymentID+"."+domain)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed", message))
	}

	// NOTE: Set EPINIO_DONT_WAIT_FOR_DEPLOYMENT when doing development to let
	// the installation continue. You can use the `make patch-epinio-deployment` target
	// later to fix the failing deployment. See also docs/development.md
	if os.Getenv("EPINIO_DONT_WAIT_FOR_DEPLOYMENT") == "" {
		if err := c.WaitUntilPodBySelectorExist(ui, EpinioDeploymentID, "app.kubernetes.io/name=epinio-server", k.Timeout); err != nil {
			return errors.Wrap(err, "failed waiting Epinio epinio-server deployment to exist")
		}
		if err := c.WaitForPodBySelectorRunning(ui, EpinioDeploymentID, "app.kubernetes.io/name=epinio-server", k.Timeout); err != nil {
			return errors.Wrap(err, "failed waiting Epinio epinio-server deployment to be running")
		}
	}

	ui.Success().Msg("Epinio deployed")

	return nil
}

func (k Epinio) GetVersion() string {
	return version.Version
}

func (k Epinio) Deploy(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		EpinioDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + EpinioDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Epinio...")

	err = k.apply(c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Epinio) Upgrade(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		EpinioDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + EpinioDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Epinio...")

	return k.apply(c, ui, options, true)
}

func (k Epinio) createEpinioNamespace(c *kubernetes.Cluster, ui *termui.UI) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Create(
		context.Background(),
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: EpinioDeploymentID,
				Labels: map[string]string{
					kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
				},
			},
		},
		metav1.CreateOptions{},
	)

	return err
}

// Replaces ##current_epinio_version## with version.Version and applies the embedded yaml
func applyEpinioServerYaml(c *kubernetes.Cluster, ui *termui.UI) (string, error) {
	yamlPathOnDisk, err := helpers.ExtractFile(epinioServerYaml)
	if err != nil {
		return "", errors.New("Failed to extract embedded file: " + epinioServerYaml + " - " + err.Error())
	}
	defer os.Remove(yamlPathOnDisk)

	fileContents, err := ioutil.ReadFile(yamlPathOnDisk)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`##current_epinio_version##`)
	renderedFileContents := re.ReplaceAll(fileContents, []byte(version.Version))

	tmpFilePath, err := helpers.CreateTmpFile(string(renderedFileContents))
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFilePath)

	return helpers.Kubectl(fmt.Sprintf("apply -n %s --filename %s", EpinioDeploymentID, tmpFilePath))
}

func (k *Epinio) createIngress(c *kubernetes.Cluster, subdomain string) error {
	_, err := c.Kubectl.ExtensionsV1beta1().Ingresses(EpinioDeploymentID).Create(
		context.Background(),
		// TODO: Switch to networking v1 when we don't care about <1.18 clusters
		// Like this (which has been reverted):
		// https://github.com/epinio/epinio/commit/7721d610fdf27a79be980af522783671d3ffc198
		&v1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "epinio",
				Namespace: EpinioDeploymentID,
				Annotations: map[string]string{
					"kubernetes.io/ingress.class": "traefik",
				},
			},
			Spec: v1beta1.IngressSpec{
				Rules: []v1beta1.IngressRule{
					{
						Host: subdomain,
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									{
										Path: "/",
										Backend: v1beta1.IngressBackend{
											ServiceName: "epinio-server",
											ServicePort: intstr.IntOrString{
												Type:   intstr.Int,
												IntVal: 80,
											},
										}}}}}}}}},
		metav1.CreateOptions{},
	)

	return err
}
