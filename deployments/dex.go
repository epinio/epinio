package deployments

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Dex struct {
	Debug   bool
	Timeout time.Duration
}

var _ kubernetes.Deployment = &Dex{}

const (
	DexDeploymentID = "dex"
	dexServerYaml   = "dex/server.yaml"
	dexVersion      = "2.29.0"
)

func (k *Dex) ID() string {
	return DexDeploymentID
}

func (k *Dex) Backup(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k *Dex) Restore(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k Dex) Describe() string {
	return emoji.Sprintf(":cloud:Dex version: %s\n", dexVersion)
}

// Delete removes Dex from kubernetes cluster
func (k Dex) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Dex...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, DexDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", DexDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Dex because namespace either doesn't exist or not owned by Dex")
		return nil
	}

	// We are taking a shortcut here. When we applied the file we had to replace
	// ##current_dex_version## with the correct version. No need to do any parsing when
	// deleting though.
	if out, err := helpers.KubectlDeleteEmbeddedYaml(dexServerYaml, true); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", dexServerYaml, out))
	}

	message := "Deleting Dex namespace " + DexDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ctx, DexDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", DexDeploymentID)
	}

	err = c.WaitForNamespaceMissing(ctx, ui, DexDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	ui.Success().Msg("Dex removed")

	return nil
}

func (k Dex) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	if err := c.CreateNamespace(ctx, DexDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	issuer := options.GetStringNG("tls-issuer")
	if out, err := k.applyDexConfigYaml(ctx, c, ui); err != nil {
		return errors.Wrap(err, out)
	}

	domain, err := options.GetString("system_domain", TektonDeploymentID)
	if err != nil {
		return errors.Wrap(err, "Couldn't get system_domain option")
	}

	// Wait for the cert manager to be present and active. It is required
	waitForCertManagerReady(ctx, ui, c)

	// Workaround for cert-manager webhook service not being immediately ready.
	// More here: https://cert-manager.io/v1.2-docs/concepts/webhook/#webhook-connection-problems-shortly-after-cert-manager-installation
	cert := auth.CertParam{
		Name:      DexDeploymentID,
		Namespace: DexDeploymentID,
		Issuer:    issuer,
		Domain:    domain,
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
		return errors.Wrap(err, "failed trying to create the dex API server cert")
	}

	message := "Creating Dex server ingress"
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", k.createIngress(ctx, c, DexDeploymentID+"."+domain)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed", message))
	}

	if err := c.WaitUntilPodBySelectorExist(ctx, ui, DexDeploymentID, "app.kubernetes.io/name=dex-server", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Dex dex-server deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning(ctx, ui, DexDeploymentID, "app.kubernetes.io/name=dex-server", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Dex dex-server deployment to be running")
	}

	ui.Success().Msg("Dex deployed")

	return nil
}

func (k Dex) GetVersion() string {
	return dexVersion
}

func (k Dex) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(ctx, DexDeploymentID, metav1.GetOptions{})
	if err == nil {
		return errors.New("Namespace " + DexDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Dex...")

	err = k.apply(ctx, c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Dex) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		DexDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + DexDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Dex...")

	return k.apply(ctx, c, ui, options, true)
}

// Replaces ##current_dex_version## with version.Version and applies the embedded yaml
func (k Dex) applyDexConfigYaml(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) (string, error) {
	yamlPathOnDisk, err := helpers.ExtractFile(dexServerYaml)
	if err != nil {
		return "", errors.New("Failed to extract embedded file: " + dexServerYaml + " - " + err.Error())
	}
	defer os.Remove(yamlPathOnDisk)

	fileContents, err := ioutil.ReadFile(yamlPathOnDisk)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`##dex_version##`)
	renderedFileContents := re.ReplaceAll(fileContents, []byte(dexVersion))

	tmpFilePath, err := helpers.CreateTmpFile(string(renderedFileContents))
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFilePath)

	return helpers.Kubectl(fmt.Sprintf("apply -n %s --filename %s", DexDeploymentID, tmpFilePath))
}

func (k *Dex) createIngress(ctx context.Context, c *kubernetes.Cluster, subdomain string) error {
	pathTypePrefix := networkingv1.PathTypeImplementationSpecific
	_, err := c.Kubectl.NetworkingV1().Ingresses(DexDeploymentID).Create(
		ctx,
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dex",
				Namespace: DexDeploymentID,
				Annotations: map[string]string{
					"kubernetes.io/ingress.class": "traefik",
					// Traefik v1 annotations for ingress with basic auth.
					// See `assets/embedded-files/dex/server.yaml` for
					// the definition of the secret.
					"ingress.kubernetes.io/auth-type":   "basic",
					"ingress.kubernetes.io/auth-secret": "dex-api-auth-secret",
					// Traefik v2 annotation for ingress with basic auth.
					// The name of the middleware is `(namespace)-(object)@kubernetescrd`.
					"traefik.ingress.kubernetes.io/router.middlewares": DexDeploymentID + "-dex-api-auth@kubernetescrd",
					// Traefik v1/v2 tls annotations.
					"traefik.ingress.kubernetes.io/router.entrypoints": "websecure",
					"traefik.ingress.kubernetes.io/router.tls":         "true",
				},
				Labels: map[string]string{
					"app.kubernetes.io/name": "dex",
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
												Name: "dex-server",
												Port: networkingv1.ServiceBackendPort{
													Number: 80,
												},
											},
										}}}}}}},
				TLS: []networkingv1.IngressTLS{{
					Hosts:      []string{subdomain},
					SecretName: "dex-tls",
				}},
			}},
		metav1.CreateOptions{},
	)

	return err
}
