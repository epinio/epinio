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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Eirini struct {
	Debug   bool
	Timeout int
}

const (
	eiriniDeploymentID = "eirini"
	eiriniVersion      = "2.0.0"
	eiriniReleasePath  = "eirini/eirini-v2.0.0.tgz" // Embedded from: https://github.com/cloudfoundry-incubator/eirini-release/releases/download/v2.0.0/eirini-yaml.tgz
	eiriniQuarksYaml   = "eirini/quarks-secrets.yaml"
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
	return eiriniDeploymentID
}

func (k *Eirini) Backup(c kubernetes.Cluster, d string) error {
	return nil
}

func (k *Eirini) Restore(c kubernetes.Cluster, d string) error {
	return nil
}

func (k Eirini) Describe() string {
	return emoji.Sprintf(":cloud:Eirini version: %s\n:clipboard:Eirini chart: %s", eiriniVersion, eiriniReleasePath)
}

func (k Eirini) Delete(c kubernetes.Cluster) error {
	return c.Kubectl.CoreV1().Namespaces().Delete(context.Background(), eiriniDeploymentID, metav1.DeleteOptions{})
}

func (k Eirini) apply(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {
	releaseDir, err := ioutil.TempDir("", "carrier")
	if err != nil {
		return err
	}
	defer os.RemoveAll(releaseDir)

	releaseFile, err := helpers.ExtractFile(eiriniReleasePath)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + eiriniReleasePath + " - " + err.Error())
	}
	defer os.Remove(releaseFile)

	err = helpers.Untar(releaseFile, releaseDir)
	if err != nil {
		return err
	}

	message := "Creating Eirini namespace for core components"
	out, err := helpers.SpinnerWaitCommand(message,
		func() (string, error) {
			file := path.Join(releaseDir, "deploy", "core", "namespace.yml")
			return helpers.Kubectl("apply -f " + file)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	message = "Creating Eirini namespace for workloads"
	out, err = helpers.SpinnerWaitCommand(message,
		func() (string, error) {
			file := path.Join(releaseDir, "deploy", "workloads", "namespace.yml")
			return helpers.Kubectl("apply -f " + file)

		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	if err = k.patchNamespaceForQuarks(c, "eirini-workloads"); err != nil {
		return err
	}
	if err = k.patchNamespaceForQuarks(c, "eirini-core"); err != nil {
		return err
	}

	message = "Creating eirini certs using quarks-secret"
	out, err = helpers.SpinnerWaitCommand(message,
		func() (string, error) {
			return helpers.KubectlApplyEmbeddedYaml(eiriniQuarksYaml)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	for _, component := range []string{"core", "events", "metrics", "workloads"} {
		message := "Deploying eirini " + component
		out, err := helpers.SpinnerWaitCommand(message,
			func() (string, error) {
				dir := path.Join(releaseDir, "deploy", component)
				return helpers.Kubectl("apply -f " + dir)
			},
		)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
		}
	}

	domain, err := options.GetString("system_domain", eiriniDeploymentID)
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

	if err := c.WaitUntilPodBySelectorExist("eirini-core", "name=eirini-api", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Eirini api deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning("eirini-core", "name=eirini-api", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Eirini api deployment to come up")
	}
	if err := c.WaitUntilPodBySelectorExist("eirini-core", "name=eirini-metrics", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Eirini metrics deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning("eirini-core", "name=eirini-metrics", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Eirini metrics deployment to come up")
	}

	emoji.Println(":heavy_check_mark: Eirini deployed")

	return nil
}

func (k Eirini) GetVersion() string {
	return eiriniVersion
}

func (k Eirini) Deploy(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		eiriniDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + eiriniDeploymentID + " present already")
	}

	emoji.Println(":ship:Deploying Eirini")
	err = k.apply(c, options)
	if err != nil {
		return err
	}

	return nil
}

func (k Eirini) Upgrade(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		eiriniDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + eiriniDeploymentID + " not present")
	}

	emoji.Println(":ship:Upgrade Eirini")
	return k.apply(c, options)
}

func (k Eirini) createClusterRegistryCredsSecret(c kubernetes.Cluster, http bool) error {
	var protocol, secretName string
	if http {
		protocol = "http"
		secretName = "cluster-registry-creds"
	} else {
		protocol = "https"
		secretName = "cluster-registry-creds-http"
	}
	_, err := c.Kubectl.CoreV1().Secrets("eirini-workloads").Create(context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
			StringData: map[string]string{
				".dockerconfigjson": `{"auths":{"` + protocol + `://127.0.0.1:30501":{"auth": "YWRtaW46cGFzc3dvcmQ=", "username":"admin","password":"password"}}}`,
			},
			Type: "kubernetes.io/dockerconfigjson",
		}, metav1.CreateOptions{})

	if err != nil {
		return err
	}
	return nil
}

func (k Eirini) createGitCredsSecret(c kubernetes.Cluster, domain string) error {
	_, err := c.Kubectl.CoreV1().Secrets("eirini-workloads").Create(context.Background(),
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "git-creds",
				Annotations: map[string]string{
					"kpack.io/git": fmt.Sprintf("http://%s.%s", giteaDeploymentID, domain),
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

func (k Eirini) patchServiceAccountWithSecretAccess(c kubernetes.Cluster, name string) error {
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

func (k Eirini) patchNamespaceForQuarks(c kubernetes.Cluster, namespace string) error {
	patchContents := `{ "metadata": { "labels": { "quarks.cloudfoundry.org/monitored": "quarks-secret" } } }`

	_, err := c.Kubectl.CoreV1().Namespaces().Patch(context.Background(), namespace,
		types.StrategicMergePatchType, []byte(patchContents), metav1.PatchOptions{})

	if err != nil {
		return err
	}
	return nil
}
