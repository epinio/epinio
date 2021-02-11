package deployments

import (
	"context"
	"fmt"

	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"github.com/suse/carrier/cli/helpers"
	"github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/paas/ui"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Workloads struct {
	Debug   bool
	Timeout int
}

const (
	WorkloadsDeploymentID   = "carrier-workloads"
	WorkloadsIngressVersion = "0.1"
	appIngressYamlPath      = "app-ingress.yaml"
)

func (k *Workloads) ID() string {
	return WorkloadsDeploymentID
}

func (k *Workloads) Backup(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k *Workloads) Restore(c *kubernetes.Cluster, ui *ui.UI, d string) error {
	return nil
}

func (k Workloads) Describe() string {
	return emoji.Sprintf(":cloud:Workloads Eirinix Ingress Version: %s\n", WorkloadsIngressVersion)
}

// Delete removes Workloads from kubernetes cluster
func (w Workloads) Delete(c *kubernetes.Cluster, ui *ui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Workloads...")

	if err := w.deleteWorkloadsNamespace(c, ui); err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", WorkloadsDeploymentID)
	}

	if out, err := helpers.KubectlDeleteEmbeddedYaml(appIngressYamlPath, true); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", appIngressYamlPath, out))
	}

	ui.Success().Msg("Workloads removed")

	return nil
}

func (w Workloads) apply(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	if err := w.createWorkloadsNamespace(c, ui); err != nil {
		return err
	}

	if out, err := helpers.KubectlApplyEmbeddedYaml(appIngressYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", appIngressYamlPath, out))
	}

	if err := c.WaitUntilPodBySelectorExist(ui, "app-ingress", "name=app-ingress", w.Timeout); err != nil {
		return errors.Wrap(err, "failed waiting app-ingress deployment to exist")
	}

	ui.Success().Msg("Workloads deployed")

	return nil
}

func (k Workloads) GetVersion() string {
	// TODO: Maybe this should be the Carrier version itself?
	return WorkloadsIngressVersion
}

func (k Workloads) Deploy(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		WorkloadsDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + WorkloadsDeploymentID + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Workloads...")

	err = k.apply(c, ui, options)
	if err != nil {
		return err
	}

	return nil
}

func (k Workloads) Upgrade(c *kubernetes.Cluster, ui *ui.UI, options kubernetes.InstallationOptions) error {
	// NOTE: Not implemented yet
	return nil
}

func (w Workloads) createWorkloadsNamespace(c *kubernetes.Cluster, ui *ui.UI) error {
	if _, err := c.Kubectl.CoreV1().Namespaces().Create(
		context.Background(),
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: WorkloadsDeploymentID,
				Labels: map[string]string{
					"quarks.cloudfoundry.org/monitored":  "quarks-secret",
					kubernetes.CarrierDeploymentLabelKey: kubernetes.CarrierDeploymentLabelValue,
				},
			},
		},
		metav1.CreateOptions{},
	); err != nil {
		return nil
	}

	return c.LabelNamespace(WorkloadsDeploymentID, kubernetes.CarrierDeploymentLabelKey, kubernetes.CarrierDeploymentLabelValue)
}

func (w Workloads) deleteWorkloadsNamespace(c *kubernetes.Cluster, ui *ui.UI) error {
	message := "Deleting Workloads namespace " + WorkloadsDeploymentID
	warning, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return c.DeleteNamespaceIfOwned(WorkloadsDeploymentID)
		},
	)
	if err != nil {
		return err
	}
	if warning != "" {
		ui.Exclamation().Msg(warning)
	}

	message = "Waiting for workloads namespace to be gone"
	warning, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			var err error
			for err == nil {
				_, err = c.Kubectl.CoreV1().Namespaces().Get(
					context.Background(),
					WorkloadsDeploymentID,
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

	return nil
}
