package deployments

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/version"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Epinio struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &Epinio{}

const (
	EpinioDeploymentID = "epinio"
	epinioServerYaml   = "epinio/server.yaml"
	epinioRolesYAML    = "epinio/roles.yaml"
	applicationCRDYaml = "epinio/app-crd.yaml"
)

func (k Epinio) ID() string {
	return EpinioDeploymentID
}

func (k Epinio) Describe() string {
	return emoji.Sprintf(":cloud:Epinio version: %s\n", version.Version)
}

// Delete removes Epinio from kubernetes cluster
func (k Epinio) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Epinio...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, EpinioDeploymentID)
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

	if out, err := helpers.KubectlDeleteEmbeddedYaml(epinioRolesYAML, true); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", epinioRolesYAML, out))
	}

	message := "Deleting Epinio namespace " + EpinioDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ctx, EpinioDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", EpinioDeploymentID)
	}

	return nil
}

func (k Epinio) PostDeleteCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	err := c.WaitForNamespaceMissing(ctx, ui, EpinioDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	ui.Success().Msg("Epinio removed")

	return nil
}

func (k Epinio) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, _ bool) error {
	if err := c.CreateNamespace(ctx, EpinioDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	apiUser, err := options.GetString("user", "")
	if err != nil {
		return err
	}

	apiPassword, err := options.GetString("password", "")
	if err != nil {
		return err
	}

	authAPI := auth.PasswordAuth{
		Username: apiUser,
		Password: apiPassword,
	}

	// Create a BasicAuth secret for our default user
	authSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-epinio-user",
			Labels: map[string]string{
				kubernetes.EpinioAPISecretLabelKey: kubernetes.EpinioAPISecretLabelValue,
			},
		},
		StringData: map[string]string{
			"username": authAPI.Username,
			"password": authAPI.Password,
		},
		Type: "BasicAuth",
	}
	_, err = c.Kubectl.CoreV1().Secrets(EpinioDeploymentID).Create(ctx, &authSecret, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "Couldn't create secret for default Epinio API user")
	}

	issuer := options.GetStringNG("tls-issuer")
	accessControlAllowOrigin := options.GetStringNG("access-control-allow-origin")
	registryTLS := options.GetBoolNG("force-kube-internal-registry-tls")

	if out, err := k.applyEpinioConfigYaml(ctx, c, ui, issuer, accessControlAllowOrigin, registryTLS); err != nil {
		return errors.Wrap(err, out)
	}

	domain, err := options.GetString("system_domain", TektonDeploymentID)
	if err != nil {
		return errors.Wrap(err, "Couldn't get system_domain option")
	}

	// Wait for the cert manager to be present and active. It is required
	if err := waitForCertManagerReady(ctx, ui, c, issuer); err != nil {
		return errors.Wrap(err, "waiting for cert manager to be ready")
	}

	// Workaround for cert-manager webhook service not being immediately ready.
	// More here: https://cert-manager.io/v1.2-docs/concepts/webhook/#webhook-connection-problems-shortly-after-cert-manager-installation
	cert := auth.CertParam{
		Name:      EpinioDeploymentID,
		Namespace: EpinioDeploymentID,
		Issuer:    issuer,
		Domain:    fmt.Sprintf("%s.%s", EpinioDeploymentID, domain),
		Labels:    map[string]string{},
	}
	err = retry.Do(func() error {
		return auth.CreateCertificate(ctx, c, cert, nil)
	},
		retry.RetryIf(func(err error) bool {
			return strings.Contains(err.Error(), " x509: ") ||
				strings.Contains(err.Error(), "failed calling webhook") ||
				strings.Contains(err.Error(), "EOF")
		}),
		retry.OnRetry(func(n uint, err error) {
			ui.Note().Msgf("Retrying creation of API cert via cert-manager (%d/%d)", n, duration.RetryMax)
		}),
		retry.Delay(5*time.Second),
		retry.Attempts(duration.RetryMax),
	)
	if err != nil {
		return errors.Wrap(err, "failed trying to create the epinio API server cert")
	}

	// Wait until the cert is there before we create the Ingress
	if _, err := c.WaitForSecret(ctx, EpinioDeploymentID, "epinio-tls", duration.ToSecretCopied()); err != nil {
		return errors.Wrap(err, "waiting for the Epinio tls certificate to be created")
	}

	message := "Creating Epinio server ingress"
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", k.createIngress(ctx, c, EpinioDeploymentID+"."+domain)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed", message))
	}

	if out, err := helpers.KubectlApplyEmbeddedYaml(applicationCRDYaml); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", applicationCRDYaml, out))
	}

	// NOTE: Set EPINIO_DONT_WAIT_FOR_DEPLOYMENT when doing development to let
	// the installation continue. You can use the `make patch-epinio-deployment` target
	// later to fix the failing deployment. See also docs/development.md
	if os.Getenv("EPINIO_DONT_WAIT_FOR_DEPLOYMENT") == "" {
		if err := c.WaitForPodBySelector(ctx, ui, EpinioDeploymentID, "app.kubernetes.io/name=epinio-server", k.Timeout); err != nil {
			return errors.Wrap(err, "failed waiting Epinio epinio-server deployment to be running")
		}
	}

	ui.Success().Msg("Epinio deployed")

	return nil
}

func (k Epinio) GetVersion() string {
	return version.Version
}

func (k Epinio) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

func (k Epinio) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(ctx, EpinioDeploymentID, metav1.GetOptions{})
	if err == nil {
		return errors.New("Namespace " + EpinioDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Epinio...")

	err = k.apply(ctx, c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Epinio) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		EpinioDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + EpinioDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Epinio...")

	return k.apply(ctx, c, ui, options, true)
}

// Replaces ##current_epinio_version## with version.Version and applies the embedded yaml
func (k Epinio) applyEpinioConfigYaml(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, issuer, accessControlAllowOrigin string, registryTLS bool) (string, error) {
	yamlPathOnDisk, err := helpers.ExtractFile(epinioServerYaml)

	if err != nil {
		return "", errors.New("Failed to extract embedded file: " + epinioServerYaml + " - " + err.Error())
	}
	defer os.Remove(yamlPathOnDisk)

	fileContents, err := ioutil.ReadFile(yamlPathOnDisk)
	if err != nil {
		return "", err
	}

	randomBytes := make([]byte, 20)
	_, err = rand.Read(randomBytes)
	if err != nil {
		return "", errors.Wrap(err, "generating a random session key")
	}
	sessionKey := base64.URLEncoding.EncodeToString(randomBytes)
	re := regexp.MustCompile(`##epinio_session_key##`)
	renderedFileContents := re.ReplaceAll(fileContents, []byte(sessionKey))

	re = regexp.MustCompile(`##current_epinio_version##`)
	renderedFileContents = re.ReplaceAll(renderedFileContents, []byte(version.Version))

	re = regexp.MustCompile(`##tls_issuer##`)
	renderedFileContents = re.ReplaceAll(renderedFileContents, []byte(issuer))

	re = regexp.MustCompile(`##force_kube_internal_registry_tls##`)
	renderedFileContents = re.ReplaceAll(renderedFileContents, []byte(strconv.FormatBool(registryTLS)))

	re = regexp.MustCompile(`##access_control_allow_origin##`)
	renderedFileContents = re.ReplaceAll(renderedFileContents, []byte(accessControlAllowOrigin))

	re = regexp.MustCompile(`##trace_level##`)
	renderedFileContents = re.ReplaceAll(renderedFileContents, []byte(strconv.Itoa(viper.GetInt("trace-level"))))
	re = regexp.MustCompile(`##epinio_timeout_multiplier##`)
	renderedFileContents = re.ReplaceAll(renderedFileContents, []byte(strconv.Itoa(viper.GetInt("timeout-multiplier"))))

	tmpFilePath, err := helpers.CreateTmpFile(string(renderedFileContents))
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFilePath)

	if out, err := helpers.Kubectl("apply",
		"--namespace", EpinioDeploymentID,
		"--filename", tmpFilePath); err != nil {
		return out, err
	}

	err = c.WaitForNamespace(ctx, ui, TektonStagingNamespace, k.Timeout)
	if err != nil {
		return "", errors.Wrapf(err, "failed to wait for %s namespace", TektonStagingNamespace)
	}

	yamlPathOnDisk, err = helpers.ExtractFile(epinioRolesYAML)
	if err != nil {
		return "", errors.New("Failed to extract embedded file: " + epinioRolesYAML + " - " + err.Error())
	}
	defer os.Remove(yamlPathOnDisk)

	return helpers.Kubectl("apply",
		"--namespace", TektonStagingNamespace,
		"--filename", yamlPathOnDisk)
}

func (k *Epinio) createIngress(ctx context.Context, c *kubernetes.Cluster, subdomain string) error {
	pathTypePrefix := networkingv1.PathTypeImplementationSpecific
	_, err := c.Kubectl.NetworkingV1().Ingresses(EpinioDeploymentID).Create(ctx,
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "epinio",
				Namespace: EpinioDeploymentID,
				Annotations: map[string]string{
					"kubernetes.io/ingress.class": "traefik",
					// Traefik v1/v2 tls annotations.
					"traefik.ingress.kubernetes.io/router.entrypoints": "websecure",
					"traefik.ingress.kubernetes.io/router.tls":         "true",
				},
				Labels: map[string]string{
					"app.kubernetes.io/name": "epinio",
				},
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						Host: subdomain,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathTypePrefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "epinio-server",
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										}}}}}}},
				TLS: []networkingv1.IngressTLS{{
					Hosts:      []string{subdomain},
					SecretName: EpinioDeploymentID + "-tls",
				}},
			}},
		metav1.CreateOptions{},
	)

	return err
}
