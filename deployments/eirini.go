package deployments

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"github.com/suse/carrier/cli/helpers"
	"github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/paas/ui"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Eirini struct {
	Debug   bool
	Timeout int
}

const (
	EiriniDeploymentID       = "eirini"
	eiriniVersion            = "2.0.0"
	eiriniReleasePath        = "eirini/eirini-v2.0.0.tgz" // Embedded from: https://github.com/cloudfoundry-incubator/eirini-release/releases/download/v2.0.0/eirini-yaml.tgz
	eiriniQuarksYaml         = "eirini/quarks-secrets.yaml"
	eiriniWorkLoadsNamespace = "eirini-workloads"
	eiriniCoreNamespace      = "eirini-core"
	eiriniIngressNamespace   = "eirini-ingress"
	eiriniIngressYaml        = "eirini/routing.yaml"
)

func (k *Eirini) NeededOptions() kubernetes.InstallationOptions {
	return kubernetes.InstallationOptions{
		{
			Name:        "system_domain",
			Description: "The domain you are planning to use for Carrier. Should be pointing to the traefik public IP",
			Type:        kubernetes.StringType,
			Default:     "",
		},
	}
}

func (k *Eirini) ID() string {
	return EiriniDeploymentID
}

func (k *Eirini) Backup(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k *Eirini) Restore(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k Eirini) Describe() string {
	return emoji.Sprintf(":cloud:Eirini version: %s\n:clipboard:Eirini chart: %s", eiriniVersion, eiriniReleasePath)
}

// Delete removes Eirini from kubernetes cluster
func (k Eirini) Delete(c *kubernetes.Cluster, ui *ui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Eirini...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(eiriniCoreNamespace)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", eiriniCoreNamespace)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Eirini because namespace either doesn't exist or not owned by Carrier")
		return nil
	}

	releaseDir, err := k.ExtractRelease()
	if err != nil {
		return err
	}
	defer func() {
		err := os.RemoveAll(releaseDir)
		if err != nil {
			ui.Exclamation().Msg(err.Error())
		}
	}()

	for _, component := range []string{
		"core/lrp-crd.yml",
		"core/task-crd.yml",
		"core/controller-deployment.yml",
		"core/controller-rbac.yml",
		"core/api-configmap.yml",
		"workloads/app-rbac.yml",
		"workloads/core/controller-rbac.yml",
	} {
		message := "Removing Eirini " + component
		out, err := helpers.WaitForCommandCompletion(ui, message,
			func() (string, error) {
				dir := path.Join(releaseDir, "deploy", component)
				return helpers.Kubectl("delete --ignore-not-found=true --wait=false -f " + dir)
			},
		)
		if err != nil {
			return errors.Wrapf(err, "%s failed:\n%s", message, out)
		}
	}

	// Delete namespaces last
	for _, namespace := range []string{eiriniCoreNamespace, eiriniWorkLoadsNamespace, eiriniIngressNamespace} {
		message := "Deleting Eirini namespace " + namespace
		_, err := helpers.WaitForCommandCompletion(ui, message,
			func() (string, error) {
				return "", c.DeleteNamespace(namespace)
			},
		)
		if err != nil {
			return errors.Wrapf(err, "Failed deleting namespace %s", namespace)
		}
	}

	message := "Waiting for Eirini workloads namespace to be gone"
	warning, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			var err error
			for err == nil {
				_, err = c.Kubectl.CoreV1().Namespaces().Get(
					context.Background(),
					"eirini-workloads",
					metav1.GetOptions{},
				)
			}
			if serr, ok := err.(*apierrors.StatusError); ok {
				if serr.ErrStatus.Reason == metav1.StatusReasonNotFound {
					return "", nil
				}
			}

			return "", err
		},
	)
	if err != nil {
		return err
	}
	if warning != "" {
		ui.Exclamation().Msg(warning)
	}

	ui.Success().Msg("Eirini removed")

	return nil
}

func (k Eirini) apply(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	releaseDir, err := k.ExtractRelease()
	if err != nil {
		return err
	}
	defer func() {
		err := os.RemoveAll(releaseDir)
		if err != nil {
			ui.Exclamation().Msg(err.Error())
		}
	}()

	message := "Creating Eirini namespace for core components"
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			file := path.Join(releaseDir, "deploy", "core", "namespace.yml")
			return helpers.Kubectl("apply -f " + file)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "%s failed:\n%s", message, out)
	}
	err = c.LabelNamespace(eiriniCoreNamespace, kubernetes.CarrierDeploymentLabelKey, kubernetes.CarrierDeploymentLabelValue)
	if err != nil {
		return err
	}

	message = "Creating Eirini namespace for workloads"
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			file := path.Join(releaseDir, "deploy", "workloads", "namespace.yml")
			return helpers.Kubectl("apply -f " + file)

		},
	)
	if err != nil {
		return errors.Wrapf(err, "%s failed:\n%s", message, out)
	}
	err = c.LabelNamespace(eiriniWorkLoadsNamespace, kubernetes.CarrierDeploymentLabelKey, kubernetes.CarrierDeploymentLabelValue)
	if err != nil {
		return err
	}

	if err = k.patchNamespaceForQuarks(c, "eirini-workloads"); err != nil {
		return err
	}
	if err = k.patchNamespaceForQuarks(c, "eirini-core"); err != nil {
		return err
	}

	message = "Creating Eirini certs using quarks-secret"
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.KubectlApplyEmbeddedYaml(eiriniQuarksYaml)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "%s failed:\n%s", message, out)
	}

	message = "Adding private registry configuration"
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return k.patchOpiConfForPrivateRegistry(path.Join(releaseDir, "deploy", "core", "api-configmap.yml"))
		},
	)
	if err != nil {
		return errors.Wrapf(err, "%s failed:\n%s", message, out)
	}

	for _, component := range []string{
		"core/lrp-crd.yml",
		"core/task-crd.yml",
		"core/controller-deployment.yml",
		"core/controller-rbac.yml",
		"core/api-configmap.yml",
		"workloads/app-rbac.yml",
		"workloads/core/controller-rbac.yml",
	} {
		message := "Deploying Eirini " + component
		out, err := helpers.WaitForCommandCompletion(ui, message,
			func() (string, error) {
				dir := path.Join(releaseDir, "deploy", component)
				return helpers.Kubectl("apply -f " + dir)
			},
		)
		if err != nil {
			return errors.Wrapf(err, "%s failed:\n%s", message, out)
		}
	}

	message = "Deploying Eirini ingress extension"
	out, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.KubectlApplyEmbeddedYaml(eiriniIngressYaml)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "%s failed:\n%s", message, out)
	}
	err = c.LabelNamespace(eiriniIngressNamespace, kubernetes.CarrierDeploymentLabelKey, kubernetes.CarrierDeploymentLabelValue)
	if err != nil {
		return err
	}

	domain, err := options.GetString("system_domain", EiriniDeploymentID)
	if err != nil {
		return err
	}
	// Create secrets and services accounts
	if err = k.createClusterRegistryCredsSecret(c, false); err != nil {
		return err
	}
	if err = k.createClusterRegistryCredsSecret(c, true); err != nil {
		return err
	}
	if err = k.createGitCredsSecret(c, domain); err != nil {
		return err
	}
	if err = k.patchServiceAccountWithSecretAccess(c, "eirini"); err != nil {
		return err
	}

	if err := c.WaitUntilPodBySelectorExist(ui, "eirini-core", "name=eirini-controller", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Eirini eirini-controller deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning(ui, "eirini-core", "name=eirini-controller", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Eirini eirini-controller deployment to come up")
	}

	ui.Success().Msg("Eirini deployed")

	return nil
}

func (k Eirini) GetVersion() string {
	return eiriniVersion
}

func (k Eirini) Deploy(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		EiriniDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + EiriniDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Eirini...")

	err = k.apply(c, ui, options)
	if err != nil {
		return err
	}

	return nil
}

func (k Eirini) Upgrade(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		EiriniDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + EiriniDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Eirini...")

	return k.apply(c, ui, options)
}

func (k Eirini) createClusterRegistryCredsSecret(c *kubernetes.Cluster, http bool) error {
	var protocol, secretName, port string
	if http {
		protocol = "http"
		secretName = "cluster-registry-creds"
		port = "30501"
	} else {
		protocol = "https"
		secretName = "cluster-registry-creds-http"
		port = "30500"
	}
	_, err := c.Kubectl.CoreV1().Secrets("eirini-workloads").Create(context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
			StringData: map[string]string{
				".dockerconfigjson": `{"auths":{"` + protocol + `://127.0.0.1:` + port + `":{"auth": "YWRtaW46cGFzc3dvcmQ=", "username":"admin","password":"password"}}}`,
			},
			Type: "kubernetes.io/dockerconfigjson",
		}, metav1.CreateOptions{})

	if err != nil {
		return err
	}
	return nil
}

func (k Eirini) createGitCredsSecret(c *kubernetes.Cluster, domain string) error {
	_, err := c.Kubectl.CoreV1().Secrets("eirini-workloads").Create(context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "git-creds",
				Annotations: map[string]string{
					"kpack.io/git": fmt.Sprintf("http://%s.%s", GiteaDeploymentID, domain),
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

func (k Eirini) patchServiceAccountWithSecretAccess(c *kubernetes.Cluster, name string) error {
	patchContents := `
{
  "secrets": [
    { "name": "cluster-registry-creds" },
    { "name": "cluster-registry-creds-http" },
    { "name": "git-creds" }
  ],
  "imagePullSecrets": [
    { "name": "cluster-registry-creds" },
    { "name": "cluster-registry-creds-http" },
    { "name": "git-creds" }
  ]
}
`
	_, err := c.Kubectl.CoreV1().ServiceAccounts("eirini-workloads").Patch(context.Background(), name, types.StrategicMergePatchType, []byte(patchContents), metav1.PatchOptions{})

	if err != nil {
		return err
	}
	return nil
}

func (k Eirini) patchNamespaceForQuarks(c *kubernetes.Cluster, namespace string) error {
	patchContents := `{ "metadata": { "labels": { "quarks.cloudfoundry.org/monitored": "quarks-secret" } } }`

	_, err := c.Kubectl.CoreV1().Namespaces().Patch(context.Background(), namespace,
		types.StrategicMergePatchType, []byte(patchContents), metav1.PatchOptions{})

	if err != nil {
		return err
	}
	return nil
}

func (k Eirini) patchOpiConfForPrivateRegistry(path string) (string, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = f.WriteString(`
      registry_secret_name: "cluster-registry-creds-http"
`)

	return "", err
}

// ExtractRelease extracts the embedded Eirini release tarball in a temporary
// directory. It returns the path to the directory or an error if something fails.
func (k Eirini) ExtractRelease() (string, error) {
	releaseDir, err := ioutil.TempDir("", "carrier")
	if err != nil {
		return "", err
	}

	releaseFile, err := helpers.ExtractFile(eiriniReleasePath)
	if err != nil {
		return "", errors.New("Failed to extract embedded file: " + eiriniReleasePath + " - " + err.Error())
	}
	defer os.Remove(releaseFile)

	return releaseDir, helpers.Untar(releaseFile, releaseDir)
}
