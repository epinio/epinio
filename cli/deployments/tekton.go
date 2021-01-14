package deployments

import (
	"context"
	"fmt"

	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"github.com/suse/carrier/cli/helpers"
	"github.com/suse/carrier/cli/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Tekton struct {
	Debug bool

	Timeout int
}

const (
	tektonDeploymentID            = "tekton"
	tektonPipelineReleaseYamlPath = "tekton/pipeline-v0.19.0.yaml"
	tektonDashboardYamlPath       = "tekton/dashboard-v0.11.1.yaml"
	tektonAdminRoleYamlPath       = "tekton/admin-role.yaml"
	tektonTriggersReleaseYamlPath = "tekton/triggers-v0.10.1.yaml"
	tektonTriggersYamlPath        = "tekton/triggers.yaml"
)

func (k *Tekton) NeededOptions() kubernetes.InstallationOptions {
	return kubernetes.InstallationOptions{}
}

func (k *Tekton) ID() string {
	return tektonDeploymentID
}

func (k *Tekton) Backup(c kubernetes.Cluster, d string) error {
	return nil
}

func (k *Tekton) Restore(c kubernetes.Cluster, d string) error {
	return nil
}

func (k Tekton) Describe() string {
	return emoji.Sprintf(":cloud:Tekton pipeline: %s\n:cloud:Tekton dashboard: %s\n:cloud:Tekton triggers: %s\n",
		tektonPipelineReleaseYamlPath, tektonDashboardYamlPath, tektonTriggersReleaseYamlPath)
}

func (k Tekton) Delete(c kubernetes.Cluster) error {
	return c.Kubectl.CoreV1().Namespaces().Delete(context.Background(), tektonDeploymentID, metav1.DeleteOptions{})
}

func (k Tekton) apply(c kubernetes.Cluster, options kubernetes.InstallationOptions, upgrade bool) error {
	// action := "install"
	// if upgrade {
	// 	action = "upgrade"
	// }

	if out, err := helpers.KubectlApplyEmbeddedYaml(tektonPipelineReleaseYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", tektonPipelineReleaseYamlPath, out))
	}
	if out, err := helpers.KubectlApplyEmbeddedYaml(tektonTriggersReleaseYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", tektonTriggersReleaseYamlPath, out))
	}
	if out, err := helpers.KubectlApplyEmbeddedYaml(tektonAdminRoleYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", tektonAdminRoleYamlPath, out))
	}

	for _, crd := range []string{
		"clustertasks.tekton.dev",
		"clustertriggerbindings.triggers.tekton.dev",
		"conditions.tekton.dev",
		"eventlisteners.triggers.tekton.dev",
		"pipelineresources.tekton.dev",
		"pipelineruns.tekton.dev",
		"pipelines.tekton.dev",
		"runs.tekton.dev",
		"taskruns.tekton.dev",
		"tasks.tekton.dev",
		"triggerbindings.triggers.tekton.dev",
		"triggers.triggers.tekton.dev",
		"triggertemplates.triggers.tekton.dev",
	} {
		message := fmt.Sprintf("Waiting for crd %s to be established", crd)
		out, err := helpers.SpinnerWaitCommand(message,
			func() (string, error) {
				return helpers.Kubectl("wait --for=condition=established --timeout=300s crd/" + crd)
			},
		)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
		}
	}

	message := "Waiting for tekton triggers webhook pod to be running"
	out, err := helpers.SpinnerWaitCommand(message,
		func() (string, error) {
			return helpers.Kubectl("wait --for=condition=Ready --timeout=300s -n tekton-pipelines --selector=app=tekton-triggers-webhook pod")
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	message = "Waiting for tekton pipelines webhook pod to be running"
	out, err = helpers.SpinnerWaitCommand(message,
		func() (string, error) {
			return helpers.Kubectl("wait --for=condition=Ready --timeout=300s -n tekton-pipelines --selector=app=tekton-pipelines-webhook pod")
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	message = "Installing staging pipelines and triggers"
	out, err = helpers.SpinnerWaitCommand(message,
		func() (string, error) {
			return helpers.KubectlApplyEmbeddedYaml(tektonTriggersYamlPath)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	message = "Install the tekton dashboard"
	out, err = helpers.SpinnerWaitCommand(message,
		func() (string, error) {
			return helpers.KubectlApplyEmbeddedYaml(tektonDashboardYamlPath)
		},
	)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
	}

	emoji.Println(":heavy_check_mark: Tekton deployed")

	return nil
}

func (k Tekton) GetVersion() string {
	return fmt.Sprintf("pipelines: %s, triggers %s, dashboard: %s",
		tektonPipelineReleaseYamlPath, tektonTriggersReleaseYamlPath, tektonDashboardYamlPath)
}

func (k Tekton) Deploy(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		tektonDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + tektonDeploymentID + " present already")
	}

	emoji.Println(":ship:Deploying Tekton")
	err = k.apply(c, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Tekton) Upgrade(c kubernetes.Cluster, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		context.Background(),
		tektonDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + tektonDeploymentID + " not present")
	}

	emoji.Println(":ship:Upgrade Tekton")
	return k.apply(c, options, true)
}
