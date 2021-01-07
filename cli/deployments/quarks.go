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

type Quarks struct {
	Debug bool

	Timeout int
}

const (
	quarksDeploymentID = "quarks"
	quarksVersion      = "6.1.17+0.gec409fd7"
	quarksChartURL     = "https://cloudfoundry-incubator.github.io/quarks-helm/cf-operator-6.1.17+0.gec409fd7.tgz"
)

func (k *Quarks) NeededOptions() kubernetes.InstallationOptions {
	return kubernetes.InstallationOptions{}
}

func (k *Quarks) ID() string {
	return quarksDeploymentID
}

func (k *Quarks) Backup(c kubernetes.Cluster, d string) error {
	return nil
}

func (k *Quarks) Restore(c kubernetes.Cluster, d string) error {
	return nil
}

func (k Quarks) Describe() string {
	return emoji.Sprintf(":cloud:Quarks version: %s\n:clipboard:Quarks chart: %s", quarksVersion, quarksChartURL)
}

func (k Quarks) Delete(c kubernetes.Cluster) error {
	return c.Kubectl.CoreV1().Namespaces().Delete(context.Background(), quarksDeploymentID, metav1.DeleteOptions{})
}

func (k Quarks) apply(c kubernetes.Cluster, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	currentdir, _ := os.Getwd()

	// Setup Quarks helm values
	var helmArgs []string

	helmCmd := fmt.Sprintf("helm %s quarks --create-namespace --namespace %s %s %s", action, quarksDeploymentID, quarksChartURL, strings.Join(helmArgs, " "))
	if _, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.New("Failed installing Quarks")
	}

	if err := c.WaitForPodBySelectorRunning(quarksDeploymentID, "", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Quarks deployment to come up")
	}

	emoji.Println(":heavy_check_mark: Quarks deployed")

	return nil
}

func (k Quarks) GetVersion() string {
	return quarksVersion
}

func (k Quarks) Deploy(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		quarksDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + quarksDeploymentID + " present already")
	}

	emoji.Println(":ship:Deploying Quarks")
	return k.apply(c, options, false)
}

func (k Quarks) Upgrade(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		quarksDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + quarksDeploymentID + " not present")
	}

	emoji.Println(":ship:Upgrade Quarks")
	return k.apply(c, options, true)
}
