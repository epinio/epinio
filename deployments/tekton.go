package deployments

import (
	"context"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/duration"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	typedbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
)

type Tekton struct {
	Debug      bool
	Secrets    []string
	ConfigMaps []string
	Timeout    time.Duration
}

const (
	TektonDeploymentID            = "tekton"
	tektonNamespace               = "tekton"
	TektonStagingNamespace        = "tekton-staging"
	tektonPipelineReleaseYamlPath = "tekton/pipeline-v0.23.0.yaml"
	tektonDashboardYamlPath       = "tekton/dashboard-v0.15.0.yaml"
	tektonAdminRoleYamlPath       = "tekton/admin-role.yaml"
	tektonTriggersReleaseYamlPath = "tekton/triggers-v0.12.1.yaml"
	tektonTriggersYamlPath        = "tekton/triggers.yaml"
	tektonStagingYamlPath         = "tekton/staging.yaml"
	tektonEventListenerYamlPath   = "tekton/event-listener.yaml"
)

func (k *Tekton) ID() string {
	return TektonDeploymentID
}

func (k *Tekton) Backup(c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k *Tekton) Restore(c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k Tekton) Describe() string {
	return emoji.Sprintf(":cloud:Tekton pipeline: %s\n:cloud:Tekton dashboard: %s\n:cloud:Tekton triggers: %s\n",
		tektonPipelineReleaseYamlPath, tektonDashboardYamlPath, tektonTriggersReleaseYamlPath)
}

// Delete removes Tekton from kubernetes cluster
func (k Tekton) Delete(c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Tekton...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(TektonStagingNamespace)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", TektonStagingNamespace)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Tekton staging because namespace either doesn't exist or not owned by Epinio")
		return nil
	} else {
		if out, err := helpers.KubectlDeleteEmbeddedYaml(tektonAdminRoleYamlPath, true); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", tektonAdminRoleYamlPath, out))
		}

		err = k.deleteCACertificate(c)
		if err != nil {
			return errors.Wrapf(err, "failed deleting ca-cert certificate")
		}

		message := "Deleting Tekton staging namespace " + TektonStagingNamespace
		_, err = helpers.WaitForCommandCompletion(ui, message,
			func() (string, error) {
				return "", c.DeleteNamespace(TektonStagingNamespace)
			},
		)
		if err != nil {
			return errors.Wrapf(err, "Failed deleting namespace %s", TektonStagingNamespace)
		}
	}

	err = c.WaitForNamespaceMissing(ui, TektonStagingNamespace, k.Timeout)
	if err != nil {
		return errors.Wrapf(err, "Failed waiting for namespace %s to be deleted", TektonStagingNamespace)
	}

	existsAndOwned, err = c.NamespaceExistsAndOwned(tektonNamespace)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", tektonNamespace)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Tekton because namespace either doesn't exist or not owned by Epinio")
		return nil
	} else {
		if out, err := helpers.KubectlDeleteEmbeddedYaml(tektonDashboardYamlPath, true); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", tektonDashboardYamlPath, out))
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
	}

	ui.Success().Msg("Tekton removed")

	return nil
}

func (k Tekton) apply(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {

	if err := c.CreateLabeledNamespace(tektonNamespace); err != nil {
		return err
	}

	if err := c.CreateLabeledNamespace(TektonStagingNamespace); err != nil {
		return err
	}
	if err := c.LabelNamespace(TektonStagingNamespace, "quarks.cloudfoundry.org/monitored", "quarks-secret"); err != nil {
		return err
	}

	if out, err := helpers.KubectlApplyEmbeddedYaml(tektonPipelineReleaseYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", tektonPipelineReleaseYamlPath, out))
	}
	if out, err := helpers.KubectlApplyEmbeddedYaml(tektonTriggersReleaseYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", tektonTriggersReleaseYamlPath, out))
	}
	if out, err := helpers.KubectlApplyEmbeddedYaml(tektonAdminRoleYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", tektonAdminRoleYamlPath, out))
	}

	kTimeout := strconv.Itoa(int(k.Timeout.Seconds()))

	err := c.WaitUntilPodBySelectorExist(ui, tektonNamespace, "app=tekton-triggers-webhook", k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed waiting tekton triggers webhook pod to exist")
	}
	err = c.WaitForPodBySelectorRunning(ui, tektonNamespace, "app=tekton-triggers-webhook", k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed waiting tekton triggers webhook pod to be running")
	}
	err = c.WaitUntilPodBySelectorExist(ui, tektonNamespace, "app=tekton-pipelines-webhook", k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed waiting tekton pipelines webhook pod to exist")
	}
	err = c.WaitForPodBySelectorRunning(ui, tektonNamespace, "app=tekton-pipelines-webhook", k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed waiting tekton pipelines webhook pod to be running")
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
				return helpers.Kubectl("wait --for=condition=established --timeout=" + kTimeout + "s crd/" + crd)
			},
		)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
		}
	}

	message := "Installing staging pipelines and triggers"
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.KubectlApplyEmbeddedYaml(tektonTriggersYamlPath)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	message = "Waiting for TriggerBinding to be there"
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.ExecToSuccessWithTimeout(
				func() (string, error) {
					return helpers.Kubectl(fmt.Sprintf("get triggerbinding -n %s staging-pipelinebinding", TektonStagingNamespace))
				}, k.Timeout, duration.PollInterval())
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	message = "Installing tekton event listener"
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.KubectlApplyEmbeddedYaml(tektonEventListenerYamlPath)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	if err := k.createGiteaCredsSecret(c); err != nil {
		return err
	}
	if err := k.createClusterRegistryCredsSecret(c); err != nil {
		return err
	}
	if err := k.createServiceAccountWithSecretAccess(c); err != nil {
		return err
	}

	// Wait until quarks is ready because we need it to create the secret
	if err := c.WaitUntilPodBySelectorExist(ui, QuarksDeploymentID, "name=quarks-secret", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Quarks quarks-secret deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning(ui, QuarksDeploymentID, "name=quarks-secret", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Quarks quarks-secret deployment to come up")
	}

	domain, err := options.GetString("system_domain", TektonDeploymentID)
	if err != nil {
		return errors.Wrap(err, "Couldn't get system_domain option")
	}

	if err := k.createCACertificate(c, domain); err != nil {
		return err
	}

	_, err = c.WaitForSecret(TektonStagingNamespace, "ca-cert", duration.ToServiceSecret())
	if err != nil {
		return err
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

	message = fmt.Sprintf("Creating registry certificates in %s", TektonStagingNamespace)
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			out1, err := helpers.ExecToSuccessWithTimeout(
				func() (string, error) {
					return helpers.Kubectl(fmt.Sprintf("get secret -n %s registry-tls-self-ca", TektonStagingNamespace))
				}, k.Timeout, duration.PollInterval())
			if err != nil {
				return out1, err
			}

			out2, err := helpers.ExecToSuccessWithTimeout(
				func() (string, error) {
					return helpers.Kubectl(fmt.Sprintf("get secret -n %s registry-tls-self", TektonStagingNamespace))
				}, k.Timeout, duration.PollInterval())

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

	message = "Creating Tekton dashboard ingress"
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", createTektonIngress(c, TektonDeploymentID+"."+domain)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed", message))
	}

	if err := c.WaitUntilPodBySelectorExist(ui, TektonStagingNamespace, "eventlistener=staging-listener,app.kubernetes.io/part-of=Triggers", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Tekton event listener deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning(ui, TektonStagingNamespace, "eventlistener=staging-listener,app.kubernetes.io/part-of=Triggers", k.Timeout); err != nil {
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

func (k Tekton) Deploy(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {

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

	s := ui.Progress("Warming up cluster with builder image")
	err = k.warmupBuilder(c)
	if err != nil {
		return err
	}
	s.Stop()

	return nil
}

func (k Tekton) Upgrade(c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
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
// kubectl get secret -n tekton-staging registry-tls-self -o json | jq -r '.["data"]["ca"]' | base64 -d | openssl x509 -hash -noout
// written in golang.
func getRegistryCAHash(c *kubernetes.Cluster, ui *termui.UI) (string, error) {
	secret, err := c.Kubectl.CoreV1().Secrets(TektonStagingNamespace).
		Get(context.Background(), "registry-tls-self", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	hash, err := GenerateHash(secret.Data["ca"])
	if err != nil {
		return "", err
	}

	return hash, nil
}

func applyTektonStaging(c *kubernetes.Cluster, ui *termui.UI) (string, error) {
	caHash, err := getRegistryCAHash(c, ui)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to get registry CA from %s namespace", TektonStagingNamespace)
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

	return helpers.Kubectl(fmt.Sprintf("apply -n %s --filename %s", TektonStagingNamespace, tmpFilePath))
}

func createTektonIngress(c *kubernetes.Cluster, subdomain string) error {
	_, err := c.Kubectl.ExtensionsV1beta1().Ingresses(tektonNamespace).Create(
		context.Background(),
		// TODO: Switch to networking v1 when we don't care about <1.18 clusters
		// Like this (which has been reverted):
		// https://github.com/epinio/epiniocommit/7721d610fdf27a79be980af522783671d3ffc198
		&v1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tekton-dashboard",
				Namespace: tektonNamespace,
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

func (k Tekton) createGiteaCredsSecret(c *kubernetes.Cluster) error {
	_, err := c.Kubectl.CoreV1().Secrets(TektonStagingNamespace).Create(context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "gitea-creds",
				Annotations: map[string]string{
					//"kpack.io/git": fmt.Sprintf("http://%s.%s", GiteaDeploymentID, domain),
					"tekton.dev/git-0": "http://gitea-http.gitea:10080", // TODO: Don't hardcode
				},
			},
			StringData: map[string]string{
				"username": "dev",
				"password": "changeme",
			},
			Type: "kubernetes.io/basic-auth",
		}, metav1.CreateOptions{})

	if err != nil {
		return err
	}
	return nil
}

// Adding the imagePullSecrets to the service account attached to the application
// pods, will automatically assign the same imagePullSecrets to the pods themselves:
// https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#verify-imagepullsecrets-was-added-to-pod-spec
func (k Tekton) createServiceAccountWithSecretAccess(c *kubernetes.Cluster) error {
	automountServiceAccountToken := false
	_, err := c.Kubectl.CoreV1().ServiceAccounts(TektonStagingNamespace).Create(
		context.Background(),
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: TektonStagingNamespace,
			},
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "registry-creds"},
				{Name: "gitea-creds"}, // TODO: Why does the app service account need access to these? Maybe this service account is use in other places too? (then split to another service account for apps and the other case)
			},
			AutomountServiceAccountToken: &automountServiceAccountToken,
		}, metav1.CreateOptions{})

	return err
}

func (k Tekton) createClusterRegistryCredsSecret(c *kubernetes.Cluster) error {
	// TODO: Are all of these really used? We need tekton to be able to access
	// the registry and also kubernetes (when we deploy our app deployments)
	auths := `{ "auths": {
		"https://127.0.0.1:30500":{"auth": "YWRtaW46cGFzc3dvcmQ=", "username":"admin","password":"password"},
		"http://127.0.0.1:30501":{"auth": "YWRtaW46cGFzc3dvcmQ=", "username":"admin","password":"password"},
		 "registry.epinio-registry":{"username":"admin","password":"password"},
		 "registry.epinio-registry:444":{"username":"admin","password":"password"} } }`

	_, err := c.Kubectl.CoreV1().Secrets(TektonStagingNamespace).Create(context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "registry-creds",
			},
			StringData: map[string]string{
				".dockerconfigjson": auths,
			},
			Type: "kubernetes.io/dockerconfigjson",
		}, metav1.CreateOptions{})

	if err != nil {
		return err
	}
	return nil
}

func (t Tekton) createCACertificate(c *kubernetes.Cluster, domain string) error {
	data := fmt.Sprintf(`{
		"apiVersion": "quarks.cloudfoundry.org/v1alpha1",
		"kind": "QuarksSecret",
		"metadata": {
			"name": "generate-ca-certificate",
			"namespace": "%s"
		},
		"spec": {
			"request" : {
				"certificate" : {
					"commonName" : "%s",
					"isCA" : true,
					"alternativeNames": [
						"%s"
					],
					"signerType" : "cluster"
				}
			},
			"secretName" : "ca-cert",
			"type" : "certificate"
		}
    }`, TektonStagingNamespace, domain, domain)

	decoderUnstructured := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	_, _, err := decoderUnstructured.Decode([]byte(data), nil, obj)
	if err != nil {
		return err
	}

	quarksSecretInstanceGVR := schema.GroupVersionResource{
		Group:    "quarks.cloudfoundry.org",
		Version:  "v1alpha1",
		Resource: "quarkssecrets",
	}

	dynamicClient, err := dynamic.NewForConfig(c.RestConfig)
	if err != nil {
		return err
	}
	_, err = dynamicClient.Resource(quarksSecretInstanceGVR).Namespace(TektonStagingNamespace).
		Create(context.Background(),
			obj,
			metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (k Tekton) deleteCACertificate(c *kubernetes.Cluster) error {
	quarksSecretInstanceGVR := schema.GroupVersionResource{
		Group:    "quarks.cloudfoundry.org",
		Version:  "v1alpha1",
		Resource: "quarkssecrets",
	}

	dynamicClient, err := dynamic.NewForConfig(c.RestConfig)
	if err != nil {
		return err
	}

	err = dynamicClient.Resource(quarksSecretInstanceGVR).Namespace(TektonStagingNamespace).
		Delete(context.Background(),
			"generate-ca-certificate",
			metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

// This function creates a dummy Job using the buildpack builder image
// in order to avoid pulling it the first time we an application is staged.
// TODO: This doesn't work in a multi-node cluster because it will only pull
// the image on one node. Maybe we could use a dummy daemonset for that.
func (k Tekton) warmupBuilder(c *kubernetes.Cluster) error {
	client, err := typedbatchv1.NewForConfig(c.RestConfig)
	if err != nil {
		return err
	}

	jobName := "buildpack-builder-warmup"
	var backoffLimit = int32(1)
	if _, err = client.Jobs(TektonStagingNamespace).Create(
		context.Background(),
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name: jobName,
				Labels: map[string]string{
					kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
				},
			},
			Spec: batchv1.JobSpec{
				BackoffLimit: &backoffLimit,
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:    "warmup",
								Image:   "paketobuildpacks/builder:full", // TODO: DRY this
								Command: []string{"/bin/ls"},
							}},
						RestartPolicy: "Never",
					}}}},
		metav1.CreateOptions{},
	); err != nil {
		return err
	}

	return c.WaitForJobCompleted(TektonStagingNamespace, jobName, duration.ToWarmupJobReady())
}

// -----------------------------------------------------------------------------------

func GenerateHash(certRaw []byte) (string, error) {
	cert, err := decodeOneCert(certRaw)
	if err != nil {
		return "", fmt.Errorf("failed to decode certificate\n%w", err)
	}

	hash, err := SubjectNameHash(cert)
	if err != nil {
		return "", fmt.Errorf("failed compute subject name hash for cert\n%w", err)
	}

	name := fmt.Sprintf("%08x", hash)
	return name, nil
}

// -----------------------------------------------------------------------------------
// See gh:paketo-buildpacks/ca-certificates (cacerts/certs.go) for the original code.

func decodeOneCert(raw []byte) (*x509.Certificate, error) {
	block, rest := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("failed find PEM data")
	}
	extra, rest := pem.Decode(rest)
	if extra != nil {
		return nil, errors.New("found multiple PEM blocks, expected exactly one")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certficate\n%w", err)
	}
	return cert, nil
}

// SubjectNameHash is a reimplementation of the X509_subject_name_hash
// in openssl. It computes the SHA-1 of the canonical encoding of the
// certificate's subject name and returns the 32-bit integer
// represented by the first four bytes of the hash using little-endian
// byte order.
func SubjectNameHash(cert *x509.Certificate) (uint32, error) {
	name, err := CanonicalName(cert.RawSubject)
	if err != nil {
		return 0, fmt.Errorf("failed to compute canonical subject name\n%w", err)
	}
	hasher := sha1.New()
	_, err = hasher.Write(name)
	if err != nil {
		return 0, fmt.Errorf("failed to compute sha1sum of canonical subject name\n%w", err)
	}
	sum := hasher.Sum(nil)
	return binary.LittleEndian.Uint32(sum[:4]), nil
}

// canonicalSET holds a of canonicalATVs. Suffix SET ensures it is
// marshaled as a set rather than a sequence by asn1.Marshal.
type canonicalSET []canonicalATV

// canonicalATV is similar to pkix.AttributeTypeAndValue but includes
// tag to ensure all values are marshaled as ASN.1, UTF8String values
type canonicalATV struct {
	Type  asn1.ObjectIdentifier
	Value string `asn1:"utf8"`
}

// CanonicalName accepts a DER encoded subject name and returns a
// "Canonical Encoding" matching that returned by the x509_name_canon
// function in openssl. All string values are transformed with
// CanonicalString and UTF8 encoded and the leading SEQ header is
// removed.
//
// For more information see
// https://stackoverflow.com/questions/34095440/hash-algorithm-for-certificate-crl-directory.
func CanonicalName(name []byte) ([]byte, error) {
	var origSeq pkix.RDNSequence
	_, err := asn1.Unmarshal(name, &origSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subject name\n%w", err)
	}
	var result []byte
	for _, origSet := range origSeq {
		var canonSet canonicalSET
		for _, origATV := range origSet {
			origVal, ok := origATV.Value.(string)
			if !ok {
				return nil, errors.New("got unexpected non-string value")
			}
			canonSet = append(canonSet, canonicalATV{
				Type:  origATV.Type,
				Value: CanonicalString(origVal),
			})
		}
		setBytes, err := asn1.Marshal(canonSet)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal canonical name\n%w", err)
		}
		result = append(result, setBytes...)
	}
	return result, nil
}

// CanonicalString transforms the given string. All leading and
// trailing whitespace is trimmed where whitespace is defined as a
// space, formfeed, tab, newline, carriage return, or vertical tab
// character. Any remaining sequence of one or more consecutive
// whitespace characters in replaced with a single ' '.
//
// This is a reimplementation of the asn1_string_canon in openssl
func CanonicalString(s string) string {
	s = strings.TrimLeft(s, " \f\t\n\n\v")
	s = strings.TrimRight(s, " \f\t\n\n\v")
	s = strings.ToLower(s)
	return string(regexp.MustCompile(`[[:space:]]+`).ReplaceAll([]byte(s), []byte(" ")))
}
