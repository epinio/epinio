package deployments

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"github.com/suse/carrier/cli/helpers"
	"github.com/suse/carrier/cli/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Gitea struct {
	Debug bool

	Timeout int
}

const (
	giteaDeploymentID = "gitea"
	giteaVersion      = "2.1.3"
	giteaChartURL     = "https://dl.gitea.io/charts/gitea-2.1.3.tgz"
)

func (k *Gitea) NeededOptions() kubernetes.InstallationOptions {
	return kubernetes.InstallationOptions{
		{
			Name:        "system_domain",
			Description: "The domain you are planning to use for Carrier. Should be pointing to the traefik public IP",
			Type:        kubernetes.StringType,
		},
	}
}

func (k *Gitea) ID() string {
	return giteaDeploymentID
}

func (k *Gitea) Backup(c kubernetes.Cluster, d string) error {
	return nil
}

func (k *Gitea) Restore(c kubernetes.Cluster, d string) error {
	return nil
}

func (k Gitea) Describe() string {
	return emoji.Sprintf(":cloud:Gitea version: %s\n:clipboard:Gitea chart: %s", giteaVersion, giteaChartURL)
}

func (k Gitea) Delete(c kubernetes.Cluster) error {
	return c.Kubectl.CoreV1().Namespaces().Delete(context.Background(), giteaDeploymentID, metav1.DeleteOptions{})
}

func (k Gitea) apply(c kubernetes.Cluster, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	currentdir, _ := os.Getwd()

	// Setup Gitea helm values
	var helmArgs []string

	domain, err := options.GetString("system_domain", giteaDeploymentID)
	if err != nil {
		return err
	}
	subdomain := giteaDeploymentID + "." + domain

	helmCmd := fmt.Sprintf("helm %s gitea --create-namespace --namespace %s --set %s %s %s", subdomain, action, giteaDeploymentID, giteaChartURL, strings.Join(helmArgs, " "))
	if _, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.New("Failed installing Gitea")
	}

	if err := c.WaitForPodBySelectorRunning(giteaDeploymentID, "", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Gitea deployment to come up")
	}

	emoji.Println(":heavy_check_mark: Gitea deployed")

	return nil
}

func (k Gitea) GetVersion() string {
	return giteaVersion
}

func (k Gitea) Deploy(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		giteaDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + giteaDeploymentID + " present already")
	}

	emoji.Println(":ship:Deploying Gitea")
	return k.apply(c, options, false)
}

func (k Gitea) Upgrade(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		giteaDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + giteaDeploymentID + " not present")
	}

	emoji.Println(":ship:Upgrade Gitea")
	return k.apply(c, options, true)
}
