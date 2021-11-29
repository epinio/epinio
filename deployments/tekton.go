package deployments

import (
	"context" // nolint:gosec // Required by subject hash specification
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/registry"
	"github.com/epinio/epinio/internal/s3manager"
	"github.com/go-logr/logr"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yaml2 "sigs.k8s.io/yaml"
)

type Tekton struct {
	Debug                     bool
	Secrets                   []string
	ConfigMaps                []string
	Timeout                   time.Duration
	S3ConnectionDetails       *s3manager.ConnectionDetails
	RegistryConnectionDetails *registry.ConnectionDetails
	Log                       logr.Logger
}

var _ kubernetes.Deployment = &Tekton{}

const (
	TektonDeploymentID            = "tekton"
	tektonNamespace               = "tekton-pipelines"
	TektonStagingNamespace        = "tekton-staging"
	tektonPipelineReleaseYamlPath = "tekton/pipeline-v0.28.0.yaml"
	tektonAdminRoleYamlPath       = "tekton/admin-role.yaml"
	tektonStagingYamlPath         = "tekton/buildpacks-task.yaml"
	tektonAWSYamlPath             = "tekton/aws-cli-0.2.yaml"
	tektonPipelineYamlPath        = "tekton/stage-pipeline.yaml"
	S3ConnectionDetailsSecret     = "epinio-s3-connection-details" // nolint:gosec
)

func (k Tekton) ID() string {
	return TektonDeploymentID
}

func (k Tekton) Describe() string {
	return emoji.Sprintf(":cloud:Tekton pipeline: %s\n", tektonPipelineReleaseYamlPath)
}

func (k Tekton) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

func (k Tekton) PostDeleteCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	err := c.WaitForNamespaceMissing(ctx, ui, tektonNamespace, k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed to delete namespace")
	}

	ui.Success().Msg("Tekton removed")

	return nil
}

// Delete removes Tekton from kubernetes cluster
func (k Tekton) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Tekton...")

	existsAndOwnedStaging, err := c.NamespaceExistsAndOwned(ctx, TektonStagingNamespace)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", TektonStagingNamespace)
	}
	existsAndOwnedPipelines, err := c.NamespaceExistsAndOwned(ctx, tektonNamespace)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", tektonNamespace)
	}

	if existsAndOwnedStaging {
		if out, err := helpers.KubectlDeleteEmbeddedYaml(tektonAdminRoleYamlPath, true); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", tektonAdminRoleYamlPath, out))
		}

		message := "Deleting Tekton staging namespace " + TektonStagingNamespace
		_, err = helpers.WaitForCommandCompletion(ui, message,
			func() (string, error) {
				return "", c.DeleteNamespace(ctx, TektonStagingNamespace)
			},
		)
		if err != nil {
			return errors.Wrapf(err, "Failed deleting namespace %s", TektonStagingNamespace)
		}

		err = c.WaitForNamespaceMissing(ctx, ui, TektonStagingNamespace, k.Timeout)
		if err != nil {
			return errors.Wrapf(err, "Failed waiting for namespace %s to be deleted", TektonStagingNamespace)
		}
	} else {
		ui.Exclamation().Msg("Skipping Tekton staging namespace because it either doesn't exist or not owned by Epinio")
	}

	if existsAndOwnedPipelines {
		if out, err := helpers.KubectlDeleteEmbeddedYaml(tektonPipelineReleaseYamlPath, true); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", tektonPipelineReleaseYamlPath, out))
		}

		if out, err := helpers.KubectlDeleteEmbeddedYaml(tektonAWSYamlPath, true); err != nil {
			return errors.Wrap(err, fmt.Sprintf("Deleting %s failed:\n%s", tektonAWSYamlPath, out))
		}

		message := "Deleting Tekton namespace " + tektonNamespace
		_, err = helpers.WaitForCommandCompletion(ui, message,
			func() (string, error) {
				return "", c.DeleteNamespace(ctx, tektonNamespace)
			},
		)
		if err != nil {
			return errors.Wrapf(err, "Failed deleting namespace %s", tektonNamespace)
		}
	} else {
		ui.Exclamation().Msg("Skipping Tekton pipelines namespace because it either doesn't exist or not owned by Epinio")
	}

	return nil
}

func (k Tekton) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions, _ bool) error {
	if err := c.CreateNamespace(ctx, tektonNamespace, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	if err := c.CreateNamespace(ctx, TektonStagingNamespace, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
		"kubed-registry-tls-from":           RegistryDeploymentID,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	if out, err := helpers.KubectlApplyEmbeddedYaml(tektonPipelineReleaseYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", tektonPipelineReleaseYamlPath, out))
	}
	if out, err := helpers.KubectlApplyEmbeddedYaml(tektonAdminRoleYamlPath); err != nil {
		return errors.Wrap(err, fmt.Sprintf("Installing %s failed:\n%s", tektonAdminRoleYamlPath, out))
	}

	err := c.WaitForPodBySelector(ctx, ui, tektonNamespace, "app=tekton-pipelines-webhook", k.Timeout)
	if err != nil {
		return errors.Wrap(err, "failed waiting tekton pipelines webhook pod to be running")
	}

	for _, crd := range []string{
		"clustertasks.tekton.dev",
		"conditions.tekton.dev",
		"pipelineresources.tekton.dev",
		"pipelineruns.tekton.dev",
		"pipelines.tekton.dev",
		"runs.tekton.dev",
		"taskruns.tekton.dev",
		"tasks.tekton.dev",
	} {
		if err := c.WaitForCRD(ctx, ui, crd, k.Timeout); err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed waiting for CRD %s to become available", crd))
		}
	}

	message := "Installing staging pipelines"
	// Workaround for tekton webhook service not working, despite pod and deployment being ready
	retryErr := retry.Do(
		func() error {
			out, err := helpers.WaitForCommandCompletion(ui, message,
				func() (string, error) {
					return helpers.KubectlApplyEmbeddedYaml(tektonPipelineYamlPath)
				},
			)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
			}
			return nil
		},
		retry.RetryIf(func(err error) bool {
			return helpers.Retryable(err.Error())
		}),
		retry.OnRetry(func(n uint, err error) {
			ui.Note().Msgf("retrying to apply %s", tektonPipelineYamlPath)
		}),
		retry.Delay(5*time.Second),
	)
	if retryErr != nil {
		return retryErr
	}
	retryErr = retry.Do(
		func() error {
			out, err := helpers.WaitForCommandCompletion(ui, message,
				func() (string, error) {
					return helpers.KubectlApplyEmbeddedYaml(tektonAWSYamlPath)
				},
			)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("%s failed:\n%s", message, out))
			}
			return nil
		},
		retry.RetryIf(func(err error) bool {
			return strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "EOF")
		}),
		retry.OnRetry(func(n uint, err error) {
			ui.Note().Msgf("retrying to apply %s", tektonAWSYamlPath)
		}),
		retry.Delay(5*time.Second),
	)
	if retryErr != nil {
		return retryErr
	}

	message = "applying tekton staging"
	s := ui.Progress(message)
	err = k.applyTektonStaging(ctx, c, options)
	if err != nil {
		s.Stop()
		return errors.Wrap(err, message)
	}
	s.Stop()

	// Create the secret that will be used to store and retrieve application
	// sources from the S3 compatible storage.
	if err := k.storeS3Settings(ctx, c); err != nil {
		return errors.Wrap(err, "storing the S3 options")
	}

	// Create the dockerconfigjson secret that will be used to push and pull
	// images from the Epinio registry (internal or external).
	// This secret is used as a Kubed source secret to be automatically copied to
	// all application namespaces that Kubernetes can pull application images into.
	if _, err := k.RegistryConnectionDetails.Store(ctx, c, TektonStagingNamespace, "registry-creds"); err != nil {
		return errors.Wrap(err, "storing the Registry options")
	}

	ui.Success().Msg("Tekton deployed")

	return nil
}

func (k Tekton) GetVersion() string {
	return fmt.Sprintf("pipelines: %s", tektonPipelineReleaseYamlPath)
}

func (k Tekton) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		tektonNamespace,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + tektonNamespace + " present already")
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Tekton...")

	err = k.apply(ctx, c, ui, options, false)
	if err != nil {
		return err
	}

	return nil
}

func (k Tekton) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		tektonNamespace,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + tektonNamespace + " not present")
	}

	ui.Note().Msg("Upgrading Tekton...")

	return k.apply(ctx, c, ui, options, true)
}

func (k Tekton) applyTektonStaging(ctx context.Context, c *kubernetes.Cluster, options kubernetes.InstallationOptions) error {
	yamlPathOnDisk, err := helpers.ExtractFile(tektonStagingYamlPath)
	if err != nil {
		return errors.New("Failed to extract embedded file: " + tektonStagingYamlPath + " - " + err.Error())
	}
	defer os.Remove(yamlPathOnDisk)

	fileContents, err := ioutil.ReadFile(yamlPathOnDisk)
	if err != nil {
		return err
	}

	tektonTask := &v1beta1.Task{}
	err = yaml2.Unmarshal(fileContents, tektonTask, func(opt *json.Decoder) *json.Decoder {
		opt.UseNumber()
		return opt
	})
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal task %s", string(fileContents))
	}

	clientSet, err := versioned.NewForConfig(c.RestConfig)
	if err != nil {
		return errors.Wrapf(err, "failed getting tekton Task clientSet")
	}

	_, err = clientSet.TektonV1beta1().Tasks(TektonStagingNamespace).Create(ctx, tektonTask, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed creating tekton Task")
	}

	return nil
}

// storeS3Settings stores the provides S3 settings in a Secret.
func (k Tekton) storeS3Settings(ctx context.Context, cluster *kubernetes.Cluster) error {
	_, err := s3manager.StoreConnectionDetails(ctx, cluster, TektonStagingNamespace, S3ConnectionDetailsSecret, *k.S3ConnectionDetails)

	return err
}
