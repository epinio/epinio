package deployments

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/s3manager"
	"github.com/go-logr/logr"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Minio struct {
	Debug               bool
	Timeout             time.Duration
	Log                 logr.Logger
	S3ConnectionDetails *s3manager.ConnectionDetails
}

var _ kubernetes.Deployment = &Minio{}

const (
	MinioDeploymentID    = "minio-operator"
	MinioTenantNamespace = "minio-epinio"
	MinioHostname        = "minio.minio-epinio.svc.cluster.local"
	MinioBucket          = "epinio"
	minioVersion         = "4.2.5"
	minioOperatorYAML    = "minio/operator.yaml"
	minioTenantYAML      = "minio/tenant.yaml"
)

func (k Minio) ID() string {
	return MinioDeploymentID
}

func (k Minio) Describe() string {
	return emoji.Sprintf(":cloud:Minio version: %s\n", minioVersion)
}

func (k Minio) PreDeployCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	return nil
}

func (k Minio) PostDeleteCheck(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	return nil
}

// Delete removes minio from kubernetes cluster
func (k Minio) Delete(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI) error {
	ui.Note().KeeplineUnder(1).Msg("Removing Minio...")

	existsAndOwned, err := c.NamespaceExistsAndOwned(ctx, MinioDeploymentID)
	if err != nil {
		return errors.Wrapf(err, "failed to check if namespace '%s' is owned or not", MinioDeploymentID)
	}
	if !existsAndOwned {
		ui.Exclamation().Msg("Skipping Minio because namespace either doesn't exist or not owned by Epinio")
		return nil
	}

	message := "Deleting Minio operator namespace " + MinioDeploymentID
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ctx, MinioDeploymentID)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", MinioDeploymentID)
	}

	message = "Deleting Minio tenant namespace " + MinioTenantNamespace
	_, err = helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return "", c.DeleteNamespace(ctx, MinioTenantNamespace)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Failed deleting namespace %s", MinioTenantNamespace)
	}

	err = c.WaitForNamespaceMissing(ctx, ui, MinioDeploymentID, k.Timeout)
	if err != nil {
		return errors.Wrapf(err, "Failed waiting for namespace %s to be deleted", MinioDeploymentID)
	}

	err = c.WaitForNamespaceMissing(ctx, ui, MinioTenantNamespace, k.Timeout)
	if err != nil {
		return errors.Wrapf(err, "Failed waiting for namespace %s to be deleted", MinioTenantNamespace)
	}

	ui.Success().Msg("Minio removed")

	return nil
}

func (k Minio) apply(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, _ kubernetes.InstallationOptions, _ bool) error {
	if err := c.CreateNamespace(ctx, MinioDeploymentID, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{}); err != nil {
		return err
	}

	if err := c.CreateNamespace(ctx, MinioTenantNamespace, map[string]string{
		kubernetes.EpinioDeploymentLabelKey: kubernetes.EpinioDeploymentLabelValue,
	}, map[string]string{"linkerd.io/inject": "enabled"}); err != nil {
		return err
	}

	if out, err := helpers.KubectlApplyEmbeddedYaml(minioOperatorYAML); err != nil {
		return errors.Wrapf(err, "Installing %s failed:\n%s", minioOperatorYAML, out)
	}

	// Create the tenant secret with random values
	err := k.createTenantSecret(ctx, c)
	if err != nil {
		return errors.Wrap(err, "creating the minio tenant secret")
	}

	// wait for crd to exist
	crd := "tenants.minio.min.io"
	message := fmt.Sprintf("Establish CRD %s", crd)
	out, err := helpers.WaitForCommandCompletion(ui, message,
		func() (string, error) {
			return helpers.Kubectl("wait",
				"--for", "condition=established",
				"--timeout", strconv.Itoa(int(k.Timeout.Seconds()))+"s",
				"crd/"+crd)
		},
	)
	if err != nil {
		return errors.Wrapf(err, "Waiting for CRD failed:\n%s", out)
	}

	if out, err := helpers.KubectlApplyEmbeddedYaml(minioTenantYAML); err != nil {
		return errors.Wrapf(err, "Installing %s failed:\n%s", minioTenantYAML, out)
	}

	ui.Success().Msg("Minio deployed")

	return nil
}

func (k Minio) GetVersion() string {
	return minioVersion
}

// MinioInternalConnectionSettings returns ConnectionDetails for an Epinio
// deployed minio server
func MinioInternalConnectionSettings() (*s3manager.ConnectionDetails, error) {
	key, err := randstr.Hex16()
	if err != nil {
		return nil, err
	}

	secret, err := randstr.Hex16()
	if err != nil {
		return nil, err
	}

	return s3manager.NewConnectionDetails(MinioHostname, key, secret, MinioBucket, "", false), nil
}

func (k Minio) Deploy(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	log := k.Log.WithName("Deploy")

	// Exit if using an external S3 store
	if k.S3ConnectionDetails.Endpoint != MinioHostname {
		log.Info("Not deploying minio, using existing endpoint")
		return nil
	}

	log.Info("start")
	defer log.Info("return")

	log.Info("check presence, minio namespace")

	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		MinioDeploymentID,
		metav1.GetOptions{},
	)
	if err == nil {
		return errors.New("Namespace " + MinioDeploymentID + " present already")
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	ui.Note().KeeplineUnder(1).Msg("Deploying Minio...")

	log.Info("deploying minio")

	return k.apply(ctx, c, ui, options, false)
}

func (k Minio) Upgrade(ctx context.Context, c *kubernetes.Cluster, ui *termui.UI, options kubernetes.InstallationOptions) error {
	_, err := c.Kubectl.CoreV1().Namespaces().Get(
		ctx,
		MinioDeploymentID,
		metav1.GetOptions{},
	)
	if err != nil {
		return errors.New("Namespace " + MinioDeploymentID + " not present")
	}

	ui.Note().Msg("Upgrading Minio...")

	return k.apply(ctx, c, ui, options, true)
}

func (k Minio) createTenantSecret(ctx context.Context, c *kubernetes.Cluster) error {
	_, err := c.Kubectl.CoreV1().Secrets(MinioTenantNamespace).Create(ctx,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "tenant-creds",
			},
			StringData: map[string]string{
				"accesskey": k.S3ConnectionDetails.AccessKeyID,
				"secretkey": k.S3ConnectionDetails.SecretAccessKey,
			},
			Type: "Opaque",
		}, metav1.CreateOptions{})

	return err
}
