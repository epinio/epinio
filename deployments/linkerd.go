package deployments

import (
	"context"
	"fmt"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/duration"
	"github.com/go-logr/logr"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
	linkerdUninstallJobYAML = "linkerd/uninstall-job.yaml"
	linkerdImage            = "splatform/epinio-linkerd"
)

func (k Linkerd) ID() string {
	return LinkerdDeploymentID
}

func (k Linkerd) Describe() string {
	return emoji.Sprintf(":cloud:Linkerd version: %s\n", linkerdVersion)
}

func (k Linkerd) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

func (k Linkerd) PostDeleteCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	return nil
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
	if err := k.createLinkerdJob(ctx, c, "linkerd-uninstall", "linkerd-installer",
		fmt.Sprintf("%s:%s", linkerdImage, linkerdVersion),
		"linkerd uninstall --verbose | kubectl delete -f -"); err != nil {
		return errors.Wrapf(err, "creating linkerd uninstall job failed")
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

func (k Linkerd) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, _ kubernetes.InstallationOptions, _ bool) error {
	linkerdJobName := "linkerd-install"
	if err := c.CreateNamespace(ctx, LinkerdDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	if out, err := helpers.KubectlApplyEmbeddedYaml(linkerdRolesYAML); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", linkerdUninstallJobYAML, out))
	}

	if err := k.createLinkerdJob(ctx, c, linkerdJobName, "linkerd-installer",
		fmt.Sprintf("%s:%s", linkerdImage, linkerdVersion),
		fmt.Sprintf("%s %s", "linkerd install | kubectl apply -f - && linkerd check --wait", duration.ToDeployment())); err != nil {
		return errors.Wrapf(err, "installing linkerd installation job failed")
	}

	if err := c.WaitForJobCompleted(ctx, LinkerdDeploymentID, linkerdJobName, k.Timeout); err != nil {
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

func (k Linkerd) createLinkerdJob(ctx context.Context,
	c *kubernetes.Cluster,
	jobName,
	serviceAccountName,
	imageName,
	jobCommand string) error {

	backoffLimit := int32(1)

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					Containers: []corev1.Container{
						{
							Name:            jobName,
							Image:           imageName,
							ImagePullPolicy: "IfNotPresent",
							Command: []string{
								"/bin/sh",
								"-c",
							},
							Args: []string{
								jobCommand,
							},
						},
					},
					RestartPolicy: "Never",
				},
			},
			BackoffLimit: &backoffLimit,
		},
	}

	_, err := c.Kubectl.BatchV1().Jobs(LinkerdDeploymentID).Create(ctx, &job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}
