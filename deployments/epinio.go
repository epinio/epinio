package deployments

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/GehirnInc/crypt/apr1_crypt"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/version"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
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
	epinioServerYaml    = "epinio/server.yaml"
	epinioStagingYAML   = "epinio/staging.yaml"
	epinioBasicAuthYaml = "epinio/basicauth.yaml"
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

	if out, err := helpers.KubectlDeleteEmbeddedYaml(epinioStagingYAML, true); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", epinioStagingYAML, out))
	}

	// (yyy) Note: We ignore deletion errors due to a mising
	// Middleware CRD. That indicates that a traefik v1 controller
	// was used, and the object was not applied. See also (xxx).

	if out, err := helpers.KubectlDeleteEmbeddedYaml(epinioBasicAuthYaml, true); err != nil {
		if !strings.Contains(out, `no matches for kind "Middleware"`) {
			return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", epinioServerYaml, out))
		}
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
	if err := c.CreateLabeledNamespace(EpinioDeploymentID); err != nil {
		return err
	}

	apiUser, err := options.GetOpt("user", "")
	if err != nil {
		return err
	}

	apiPassword, err := options.GetOpt("password", "")
	if err != nil {
		return err
	}

	if out, err := k.applyEpinioConfigYaml(c, ui,
		apiUser.Value.(string),
		apiPassword.Value.(string)); err != nil {
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

// Replaces ##current_epinio_version## with version.Version and applies the embedded yaml
func (k Epinio) applyEpinioConfigYaml(c *kubernetes.Cluster, ui *termui.UI, user, pass string) (string, error) {
	// (xxx) Apply traefik v2 middleware. This will fail for a
	// traefik v1 controller.  Ignore error if it was due due to a
	// missing Middleware CRD. That indicates presence of the
	// traefik v1 controller. See also (yyy).

	yamlPathOnDisk, err := helpers.ExtractFile(epinioBasicAuthYaml)
	if err != nil {
		return "", errors.New("Failed to extract embedded file: " + epinioBasicAuthYaml + " - " + err.Error())
	}
	defer os.Remove(yamlPathOnDisk)

	out, err := helpers.Kubectl(fmt.Sprintf("apply -n %s --filename %s", EpinioDeploymentID, yamlPathOnDisk))
	if err != nil && !strings.Contains(out, `no matches for kind "Middleware"`) {
		return "", err
	}

	yamlPathOnDisk, err = helpers.ExtractFile(epinioServerYaml)

	if err != nil {
		return "", errors.New("Failed to extract embedded file: " + epinioServerYaml + " - " + err.Error())
	}
	defer os.Remove(yamlPathOnDisk)

	fileContents, err := ioutil.ReadFile(yamlPathOnDisk)
	if err != nil {
		return "", err
	}

	passhash, err := hashApr1(pass)
	if err != nil {
		return "", err
	}
	credentials := fmt.Sprintf("%s:%s", user, passhash)
	encodedCredentials := base64.StdEncoding.EncodeToString([]byte(credentials))

	re := regexp.MustCompile(`##current_epinio_version##`)
	renderedFileContents := re.ReplaceAll(fileContents, []byte(version.Version))

	re = regexp.MustCompile(`##api_credentials##`)
	renderedFileContents = re.ReplaceAll(renderedFileContents, []byte(encodedCredentials))

	tmpFilePath, err := helpers.CreateTmpFile(string(renderedFileContents))
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFilePath)

	if out, err := helpers.Kubectl(fmt.Sprintf("apply -n %s --filename %s", EpinioDeploymentID, tmpFilePath)); err != nil {
		return out, err
	}

	err = c.WaitForNamespace(ui, TektonStagingNamespace, k.Timeout)
	if err != nil {
		return "", errors.Wrapf(err, "failed to wait for %s namespace", TektonStagingNamespace)
	}

	yamlPathOnDisk, err = helpers.ExtractFile(epinioStagingYAML)
	if err != nil {
		return "", errors.New("Failed to extract embedded file: " + epinioServerYaml + " - " + err.Error())
	}
	defer os.Remove(yamlPathOnDisk)

	return helpers.Kubectl(fmt.Sprintf("apply -n %s --filename %s", TektonStagingNamespace, yamlPathOnDisk))
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
					// Traefik v1 annotations for ingress with basic auth.
					// See `assets/embedded-files/epinio/server.yaml` for
					// the definition of the secret.
					"ingress.kubernetes.io/auth-type":   "basic",
					"ingress.kubernetes.io/auth-secret": "epinio-api-auth-secret",
					// Traefik v2 annotation for ingress with basic auth.
					// The name of the middleware is `(namespace)-(object)@kubernetescrd`.
					"traefik.ingress.kubernetes.io/router.middlewares": EpinioDeploymentID + "-epinio-api-auth@kubernetescrd",
				},
				Labels: map[string]string{
					"app.kubernetes.io/name": "epinio",
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

// hashApr1 generates an Apache MD5 hash for a password.
// See https://github.com/foomo/htpasswd for the origin of this code.
// MIT licensed, as per `blob/master/LICENSE.txt`
//
// The https://github.com/GehirnInc/crypt package used here is
// licensed under BSD 2-Clause "Simplified" License

func hashApr1(password string) (string, error) {
	return apr1_crypt.New().Generate([]byte(password), nil)
}
