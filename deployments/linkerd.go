package deployments

import (
	"context"
	"fmt"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/go-logr/logr"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Linkerd struct {
	Debug   bool
	Timeout time.Duration
	Log     logr.Logger
}

var _ kubernetes.Deployment = &Linkerd{}

const (
	LinkerdDeploymentID     = "linkerd"
	linkerdVersion          = "2.10.2"
	linkerdRolesYAML        = "linkerd/rbac.yaml"
	linkerdInstallJobYAML   = "linkerd/install-job.yaml"
	linkerdUninstallJobYAML = "linkerd/uninstall-job.yaml"
)

func (k *Linkerd) ID() string {
	return LinkerdDeploymentID
}

func (k *Linkerd) Backup(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k *Linkerd) Restore(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, d string) error {
	return nil
}

func (k Linkerd) Describe() string {
	return emoji.Sprintf(":cloud:Linkerd version: %s\n", linkerdVersion)
}

// Delete removes linkerd from kubernetes cluster
func (k Linkerd) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Linkerd...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, LinkerdDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", LinkerdDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Linkerd because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	// Remove linkerd with the uninstall job
	if out, err := helpers.KubectlApplyEmbeddedYaml(linkerdUninstallJobYAML); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", linkerdUninstallJobYAML, out))
	}

	// The uninstall job also deletes the namespace.
	err = c.WaitForNamespaceMissing(ctx, ui, LinkerdDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	// Now delete the service account too.
	if out, err := helpers.KubectlDeleteEmbeddedYaml(linkerdRolesYAML, true); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", linkerdUninstallJobYAML, out))
	}

	ui.Success().Msg("Linkerd removed")

	return nil
}

func (k Linkerd) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, upgrade bool) error {
	if err := c.CreateNamespace(ctx, LinkerdDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	if out, err := helpers.KubectlApplyEmbeddedYaml(linkerdRolesYAML); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", linkerdUninstallJobYAML, out))
	}

	if out, err := helpers.KubectlApplyEmbeddedYaml(linkerdInstallJobYAML); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", linkerdInstallJobYAML, out))
	}

	if err := c.WaitForJobCompleted(ctx, LinkerdDeploymentID, "linkerd-install", k.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting Linkerd install job to complete")
	}

	ui.Success().Msg("Linkerd deployed")

	return nil
}

func (k Linkerd) GetVersion() string {
	return linkerdVersion
}

func (k Linkerd) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	log := k.Log.WithName("Deploy")
	log.Info("start")
	defer log.Info("return")

	skipLinkerd, err := options.GetBool("skip-linkerd", LinkerdDeploymentID)
	if err != nil {
		return errors.Wrap(err, "Couldn't get skip-linkerd option")
	}
	if skipLinkerd {
		ui.Exclamation().Msg("Skipping Linkerd deployment by user request")
		return nil
	}

	// Cases to consider, plus actions -- Analogous to Traefik
	//
	//     | Service | Namespace | Meaning                             | Actions
	// --- | ---     | ---       | ---                                 | ---
	//  a  | yes     | yes       | Linkerd present, likely from Epinio | Nothing
	//  b  | yes     | no        | Linkerd present, likely external    | Nothing
	//  c  | no      | yes       | Something has claimed the namespace | Error
	//  d  | no      | no        | Namespace is free for use           | Deploy

	log.Info("check presence, local service")

	_, err = c.Kubectl.CoreV1().Services(LinkerdDeploymentID).Get(
		ctx,
		"linkerd-dst",
		metav1.GetOptions{},
	)
	if err == nil {
		log.Info("service present")

		ui.Exclamation().Msg("Linkerd already installed, skipping")
		return nil
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	log.Info("check presence, linkerd namespace")

	_, err = c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		LinkerdDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + LinkerdDeploymentID + " present already")
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Linkerd...")

	log.Info("deploying linkerd")

	return k.apply(ctx, c, ui, options, false)
}

func (k Linkerd) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		LinkerdDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + LinkerdDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Linkerd...")

	return k.apply(ctx, c, ui, options, true)
}
