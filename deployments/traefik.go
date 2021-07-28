package deployments

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/duration"
	"github.com/go-logr/logr"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Traefik struct {
	Debug   bool
	Timeout time.Duration
	Log     logr.Logger
}

var _ kubernetes.Deployment = &Traefik{}

const (
	TraefikDeploymentID   = "traefik"
	traefikVersion        = "9.11.0"
	traefikChartURL       = "https://helm.traefik.io/traefik/traefik-9.11.0.tgz"
	MessageLoadbalancerIP = "timed out waiting for LoadBalancer IP on traefik service\n" +
		"Ensure your kubernetes platform has the ability to provision a LoadBalancer IP address.\n\n" +
		"Follow these steps to enable this ability\n" +
		"https://github.com/epinio/epinio/blob/main/docs/user/howtos/provision_external_ip_for_local_kubernetes.md\n"
)

func (k *Traefik) ID() string {
	return TraefikDeploymentID
}

func (k *Traefik) Backup(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k *Traefik) Restore(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k Traefik) Describe() string {
	return emoji.Sprintf(":cloud:Traefik version: %s\n:clipboard:Traefik Ingress chart: %s", traefikVersion, traefikChartURL)
}

// Delete removes traefik from kubernetes cluster
func (k Traefik) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Traefik...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, TraefikDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", TraefikDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Traefik because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return errors.New("Failed uninstalling Traefik: " + err.Error())
	}

	message := "Removing helm release " + TraefikDeploymentID
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			helmCmd := fmt.Sprintf("helm uninstall traefik --namespace '%s'", TraefikDeploymentID)
			return helpers.RunProc(helmCmd, currentdir, k.Debug)
		},
	)
	if err != nil {
		if strings.Contains(out, "release: not found") {
			ui.Exclamation().Msgf("%s helm release not found, skipping.\n", TraefikDeploymentID)
		} else {
			return errors.Wrapf(err, "Failed uninstalling helm release %s: %s", TraefikDeploymentID, out)
		}
	}

	message = "Deleting Traefik namespace " + TraefikDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ctx, TraefikDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", TraefikDeploymentID)
	}

	ui.Success().Msg("Traefik removed")

	return nil
}

func (k Traefik) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	action := "install"
	if upgrade {
		action = "upgrade"
	}

	if err := c.CreateNamespace(ctx, TraefikDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, nil); err != nil {
		return err
	}

	currentdir, err := os.Getwd()
	if err != nil {
		return err
	}
	loadBalancerIP := options.GetStringNG("ingress-service-ip")

	// Setup Traefik helm values
	var helmArgs []string

	// Disable sending anonymous usage statistics
	// https://github.com/traefik/traefik-helm-chart/blob/v9.11.0/traefik/values.yaml#L170
	// Overwrite globalArguments until https://github.com/traefik/traefik-helm-chart/issues/357 is fixed
	helmArgs = append(helmArgs, `--set "globalArguments="`)
	helmArgs = append(helmArgs, `--set-string deployment.podAnnotations."linkerd\.io/inject"=enabled`)
	helmArgs = append(helmArgs, fmt.Sprintf("--set-string service.spec.loadBalancerIP=%s", loadBalancerIP))

	helmCmd := fmt.Sprintf("helm %s traefik --namespace %s %s %s", action, TraefikDeploymentID, traefikChartURL, strings.Join(helmArgs, " "))
	if out, err := helpers.RunProc(helmCmd, currentdir, k.Debug); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed installing Traefik: %s\n", out))
	}

	if err := c.WaitUntilPodBySelectorExist(ctx, ui, TraefikDeploymentID, "app.kubernetes.io/name=traefik", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting for Traefik Ingress deployment to exist")
	}
	if err := c.WaitForPodBySelectorRunning(ctx, ui, TraefikDeploymentID, "app.kubernetes.io/name=traefik", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting for Traefik Ingress deployment to come up")
	}

	if err := c.WaitUntilServiceHasLoadBalancer(ctx, ui, TraefikDeploymentID, "traefik", duration.ToServiceLoadBalancer()); err != nil {
		return errors.Wrap(err, MessageLoadbalancerIP)
	}

	ui.Success().Msg("Traefik Ingress deployed")

	return nil
}

func (k Traefik) GetVersion() string {
	return traefikVersion
}

func (k Traefik) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	log := k.Log.WithName("Deploy")
	log.Info("start")
	defer log.Info("return")

	// When called from `install` option `skip-traefik` is present.
	// When called from `install-ingress` the option is NOT present.
	// It does not make sense to skip installing the very thing the command is about.

	skipTraefik, err := options.GetBool("skip-traefik", TraefikDeploymentID)
	if err != nil {
		if err.Error() != "skip-traefik not set" {
			return errors.Wrap(err, "Couldn't get skip-traefik option")
		}

		skipTraefik = false
	}

	log.Info("config", "skipTraefik", skipTraefik)

	if skipTraefik {
		ui.Exclamation().Msg("Skipping Traefik Ingress deployment by user request")
		return nil
	}

	// Cases to consider, plus actions
	//
	//     | Service | Namespace | Meaning                             | Actions
	// --- | ---     | ---       | ---                                 | ---
	//  a  | yes     | yes       | Traefik present, likely from Epinio | Nothing
	//  b  | yes     | no        | Traefik present, likely external    | Nothing
	//  c  | no      | yes       | Something has claimed the namespace | Error
	//  d  | no      | no        | Namespace is free for use           | Deploy

	log.Info("check presence, local service")

	_, err = c.Kubectl.CoreV1().Services(TraefikDeploymentID).Get(
		ctx,
		"traefik",
		metav1.GetOptions{},
	)
	if err == nil {
		log.Info("service present")

		ui.Exclamation().Msg("Traefik Ingress already installed, skipping")
		return nil
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	log.Info("check presence, system service")

	_, err = c.Kubectl.CoreV1().Services("kube-system").Get(
		ctx,
		"traefik",
		metav1.GetOptions{},
	)
	if err == nil {
		log.Info("service present")

		ui.Exclamation().Msg("System Ingress present, skipping")
		return nil
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	log.Info("check presence, traefik namespace")

	_, err = c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		TraefikDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		log.Info("namespace present")

		return errors.New("Namespace " + TraefikDeploymentID + " present already")
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Traefik Ingress...")

	log.Info("deploying traefik")

	return k.apply(ctx, c, ui, options, false)
}

func (k Traefik) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		TraefikDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + TraefikDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Traefik Ingress...")

	return k.apply(ctx, c, ui, options, true)
}
