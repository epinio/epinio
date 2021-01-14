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
	Debug bool

	Timeout int
}

const (
	eiriniDeploymentID = "eirini"
	eiriniVersion      = "2.0.0"
	eiriniReleaseURL   = "https://github.com/cloudfoundry-incubator/eirini-release/releases/download/v2.0.0/eirini-yaml.tgz"
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
	return emoji.Sprintf(":cloud:Eirini version: %s\n:clipboard:Eirini chart: %s", eiriniVersion, eiriniReleaseURL)
}

func (k Eirini) Delete(c kubernetes.Cluster) error {
	return c.Kubectl.CoreV1().Namespaces().Delete(context.Background(), eiriniDeploymentID, metav1.DeleteOptions{})
}

func (k Eirini) apply(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {
	releaseDir, err := ioutil.TempDir("", "carrier")
	if err != nil {
		return err
	}
	defer os.Remove(releaseDir)

	// Download eirini release
	err = helpers.DownloadFile(eiriniReleaseURL, "eirini-release.tgz", releaseDir)
	if err != nil {
		return err
	}
	releaseFile := path.Join(releaseDir, "eirini-release.tgz")
	err = helpers.Untar(releaseFile, releaseDir)
	if err != nil {
		return err
	}

	// Install eirini yamls
	// TODO: !!!! Get rid of bash. Use `kubectl` to install things directly.
	cmd := "bash -c ./deploy/scripts/deploy.sh"
	if out, err := helpers.RunProc(cmd, releaseDir, k.Debug); err != nil {
		return errors.New("Failed installing Eirini: " + out)
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
	if err = k.patchWorkloadsNamespaceForQuarks(c); err != nil {
		return err
	}

	// TODO: Implement a waiting method that doesn't require a specific label
	// or exact pod name

	// if err := c.WaitForPodBySelectorRunning("eirini-core", "", k.Timeout); err != nil {
	// 	return errors.Wrap(err, "failed waiting Eirini deployment to come up")
	// }

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

func (k Eirini) patchWorkloadsNamespaceForQuarks(c kubernetes.Cluster) error {
	patchContents := `
		{ "metadata": { "labels": {
		  "quarks.cloudfoundry.org/monitored": "quarks-secret" } } }`

	_, err := c.Kubectl.CoreV1().Namespaces().Patch(context.Background(), "eirini-workloads", types.StrategicMergePatchType, []byte(patchContents), metav1.PatchOptions{})

	if err != nil {
		return err
	}
	return nil
}
