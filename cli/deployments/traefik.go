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

type Traefik struct {
	Debug bool

	Timeout int
}

const (
	traefikDeploymentID = "traefik"
	traefikVersion      = "9.11.0"
	traefikChartURL     = "https://helm.traefik.io/traefik/traefik-9.11.0.tgz"
)

func (k *Traefik) NeededOptions() kubernetes.InstallationOptions {
	return kubernetes.InstallationOptions{}
}

func (k *Traefik) ID() string {
	return traefikDeploymentID
}

func (k *Traefik) Backup(c kubernetes.Cluster, d string) error {
	return nil
}

func (k *Traefik) Restore(c kubernetes.Cluster, d string) error {
	return nil
}

func (k Traefik) Describe() string {
	return emoji.Sprintf(":cloud:Traefik version: %s\n:clipboard:Traefik Ingress chart: %s", traefikVersion, traefikChartURL)
}

func (k Traefik) Delete(c kubernetes.Cluster) error {
	return c.Kubectl.CoreV1().Namespaces().Delete(context.Background(), traefikDeploymentID, metav1.DeleteOptions{})
}

//	for i, ip := range c.GetPlatform().ExternalIPs() {
//		helmArgs = append(helmArgs, "--set controller.service.externalIPs["+strconv.Itoa(i)+"]="+ip)
func (k Traefik) apply(c kubernetes.Cluster, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	currentdir, _ := os.Getwd()

	// Setup Traefik helm values
	var helmArgs []string

	helmCmd := fmt.Sprintf("helm %s traefik --create-namespace --namespace %s %s %s", action, traefikDeploymentID, traefikChartURL, strings.Join(helmArgs, " "))
	if _, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.New("Failed installing Traefik")
	}

	if err := c.WaitForPodBySelectorRunning(traefikDeploymentID, "", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Traefik Ingress deployment to come up")
	}

	emoji.Println(":heavy_check_mark: Traefik Ingress deployed")

	return nil
}

func (k Traefik) GetVersion() string {
	return traefikVersion
}

func (k Traefik) Deploy(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		traefikDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + traefikDeploymentID + " present already")
	}

	emoji.Println(":ship:Deploying Traefik Ingress")
	return k.apply(c, options, false)
}

func (k Traefik) Upgrade(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		traefikDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + traefikDeploymentID + " not present")
	}

	emoji.Println(":ship:Upgrade Traefik Ingress")
	return k.apply(c, options, true)
}
