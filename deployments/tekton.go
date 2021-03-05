package deployments

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"github.com/suse/carrier/helpers"
	"github.com/suse/carrier/kubernetes"
	"github.com/suse/carrier/paas/ui"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type Tekton struct {
	Debug      bool
	Secrets    []string
	ConfigMaps []string
	Timeout    int
}

const (
	TektonDeploymentID            = "tekton"
	tektonNamespace               = "tekton-pipelines"
	tektonPipelineReleaseYamlPath = "tekton/pipeline-v0.19.0.yaml"
	tektonDashboardYamlPath       = "tekton/dashboard-v0.11.1.yaml"
	tektonAdminRoleYamlPath       = "tekton/admin-role.yaml"
	tektonTriggersReleaseYamlPath = "tekton/triggers-v0.11.1.yaml"
	tektonTriggersYamlPath        = "tekton/triggers.yaml"
	tektonStagingYamlPath         = "tekton/staging.yaml"
)

func (k *Tekton) ID() string {
	return TektonDeploymentID
}

func (k *Tekton) Backup(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k *Tekton) Restore(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k Tekton) Describe() string {
	return emoji.Sprintf(":cloud:Tekton pipeline: %s\n:cloud:Tekton dashboard: %s\n:cloud:Tekton triggers: %s\n",
		tektonPipelineReleaseYamlPath, tektonDashboardYamlPath, tektonTriggersReleaseYamlPath)
}

// Delete removes Tekton from kubernetes cluster
func (k Tekton) Delete(c *kubernetes.Cluster, ui *ui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Tekton...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(tektonNamespace)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", tektonNamespace)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Tekton because namespace either doesn't exist or not owned by Carrier")
		return nil
	}

	if out, err := helpers.KubectlDeleteEmbeddedYaml(tektonDashboardYamlPath, true); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", tektonDashboardYamlPath, out))
	}
	if out, err := helpers.KubectlDeleteEmbeddedYaml(tektonAdminRoleYamlPath, true); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", tektonAdminRoleYamlPath, out))
	}
	if out, err := helpers.KubectlDeleteEmbeddedYaml(tektonTriggersReleaseYamlPath, true); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", tektonTriggersReleaseYamlPath, out))
	}
	if out, err := helpers.KubectlDeleteEmbeddedYaml(tektonPipelineReleaseYamlPath, true); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", tektonPipelineReleaseYamlPath, out))
	}

	message := "Deleting Tekton namespace " + tektonNamespace
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(tektonNamespace)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", tektonNamespace)
	}

	ui.Success().Msg("Tekton removed")

	return nil
}

func (k Tekton) apply(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions, upgrade bool) error {

	// action := "install"
	// if upgrade {
	// 	action = "upgrade"
	// }

	if out, err := helpers.KubectlApplyEmbeddedYaml(tektonPipelineReleaseYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", tektonPipelineReleaseYamlPath, out))
	}
	if out, err := helpers.KubectlApplyEmbeddedYaml(tektonTriggersReleaseYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", tektonTriggersReleaseYamlPath, out))
	}
	if out, err := helpers.KubectlApplyEmbeddedYaml(tektonAdminRoleYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", tektonAdminRoleYamlPath, out))
	}

	err := c.LabelNamespace(tektonNamespace, kubernetes.CarrierDeploymentLabelKey, kubernetes.CarrierDeploymentLabelValue)
	if err != nil {
		return err
	}

	for _, crd := range []string{
		"clustertasks.tekton.dev",
		"clustertriggerbindings.triggers.tekton.dev",
		"conditions.tekton.dev",
		"eventlisteners.triggers.tekton.dev",
		"pipelineresources.tekton.dev",
		"pipelineruns.tekton.dev",
		"pipelines.tekton.dev",
		"runs.tekton.dev",
		"taskruns.tekton.dev",
		"tasks.tekton.dev",
		"triggerbindings.triggers.tekton.dev",
		"triggers.triggers.tekton.dev",
		"triggertemplates.triggers.tekton.dev",
	} {
		message := fmt.Sprintf("Establish CRD %s", crd)
		out, err := helpers.WaitForCommandCompletion(ui, message,
			func() (string, error) {
				return helpers.Kubectl("wait --for=condition=established --timeout=" + strconv.Itoa(k.Timeout) + "s crd/" + crd)
			},
		)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
		}
	}

	message := "Starting tekton triggers webhook pod"
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.Kubectl("wait --for=condition=Ready --timeout=" + strconv.Itoa(k.Timeout) + "s -n tekton-pipelines --selector=app=tekton-triggers-webhook pod")
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	message = "Starting tekton pipelines webhook pod"
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.Kubectl("wait --for=condition=Ready --timeout=" + strconv.Itoa(k.Timeout) + "s -n tekton-pipelines --selector=app=tekton-pipelines-webhook pod")
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	message = "Installing staging pipelines and triggers"
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.KubectlApplyEmbeddedYaml(tektonTriggersYamlPath)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	message = "Installing the tekton dashboard"
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.KubectlApplyEmbeddedYaml(tektonDashboardYamlPath)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	message = "Creating registry certificates in carrier-workloads"
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			out1, err := helpers.ExecToSuccessWithTimeout(
				func() (string, error) {
					return helpers.Kubectl("get secret -n carrier-workloads registry-tls-self-ca")
				}, time.Duration(k.Timeout)*time.Second, 3*time.Second)
			if err != nil {
				return out1, err
			}

			out2, err := helpers.ExecToSuccessWithTimeout(
				func() (string, error) {
					return helpers.Kubectl("get secret -n carrier-workloads registry-tls-self")
				}, time.Duration(k.Timeout)*time.Second, 3*time.Second)

			return fmt.Sprintf("%s\n%s", out1, out2), err
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	message = "Applying tekton staging resources"
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return applyTektonStaging(c, ui)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	domain, err := options.GetString("system_domain", TektonDeploymentID)
	if err != nil {
		return errors.Wrap(err, "Couldn't get system_domain option")
	}

	message = "Creating Tekton dashboard ingress"
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", createTektonIngress(c, TektonDeploymentID+"."+domain)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed", message))
	}

	if err := c.WaitUntilPodBySelectorExist(ui, WorkloadsDeploymentID, "eventlistener=staging-listener,app.kubernetes.io/part-of=Triggers", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Tekton event listener deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning(ui, WorkloadsDeploymentID, "eventlistener=staging-listener,app.kubernetes.io/part-of=Triggers", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Tekton event listener deployment to come up")
	}

	if err := c.WaitUntilPodBySelectorExist(ui, tektonNamespace, "app.kubernetes.io/name=dashboard,app.kubernetes.io/part-of=tekton-dashboard", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Tekton dashboard deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning(ui, tektonNamespace, "app.kubernetes.io/name=dashboard,app.kubernetes.io/part-of=tekton-dashboard", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Tekton dashboard deployment to come up")
	}

	ui.Success().Msg("Tekton deployed")

	return nil
}

func (k Tekton) GetVersion() string {
	return fmt.Sprintf("pipelines: %s, triggers %s, dashboard: %s",
		tektonPipelineReleaseYamlPath, tektonTriggersReleaseYamlPath, tektonDashboardYamlPath)
}

func (k Tekton) Deploy(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		TektonDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + TektonDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Tekton...")

	err = k.apply(c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Tekton) Upgrade(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		TektonDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + TektonDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Tekton...")

	return k.apply(c, ui, options, true)
}

// The equivalent of:
// kubectl get secret -n carrier-workloads registry-tls-self -o json | jq -r '.["data"]["ca"]' | base64 -d | openssl x509 -hash -noout
// written in golang.
func getRegistryCAHash(c *kubernetes.Cluster, ui *ui.UI) (string, error) {
	secret, err := c.Kubectl.CoreV1().Secrets("carrier-workloads").
		Get(context.Background(), "registry-tls-self", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return helpers.OpenSSLSubjectHash(string(secret.Data["ca"]))
}

func applyTektonStaging(c *kubernetes.Cluster, ui *ui.UI) (string, error) {
	caHash, err := getRegistryCAHash(c, ui)
	if err != nil {
		return "", errors.Wrap(err, "Failed to get registry CA from carrier-workloads namespace")
	}

	yamlPathOnDisk, err := helpers.ExtractFile(tektonStagingYamlPath)
	if err != nil {
		return "", errors.New("Failed to extract embedded file: " + tektonStagingYamlPath + " - " + err.Error())
	}
	defer os.Remove(yamlPathOnDisk)

	fileContents, err := ioutil.ReadFile(yamlPathOnDisk)
	if err != nil {
		return "", err
	}

	// Constructing the name of the cert file as required by openssl.
	// Lookup "subject_hash" in the docs: https://www.openssl.org/docs/man1.0.2/man1/x509.html
	re := regexp.MustCompile(`{{CA_SELF_HASHED_NAME}}`)
	renderedFileContents := re.ReplaceAll(fileContents, []byte(caHash+".0"))

	tmpFilePath, err := helpers.CreateTmpFile(string(renderedFileContents))
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFilePath)

	return helpers.Kubectl(fmt.Sprintf("apply -n carrier-workloads --filename %s", tmpFilePath))
}

func createTektonIngress(c *kubernetes.Cluster, subdomain string) error {
	_, err := c.Kubectl.ExtensionsV1beta1().Ingresses("tekton-pipelines").Create(
		context.Background(),
		// TODO: Switch to networking v1 when we don't care about <1.18 clusters
		// Like this (which has been reverted):
		// https://github.com/SUSE/carrier/commit/7721d610fdf27a79be980af522783671d3ffc198
		&v1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tekton-dashboard",
				Namespace: "tekton-pipelines",
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
											ServiceName: "tekton-dashboard",
											ServicePort: intstr.IntOrString{
												Type:   intstr.Int,
												IntVal: 9097,
											},
										}}}}}}}}},
		metav1.CreateOptions{},
	)

	return err
}
