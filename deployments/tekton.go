package deployments

import (
	"context"
	"crypto/sha1" // nolint:gosec // Required by subject hash specification
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/registry"
	"github.com/epinio/epinio/internal/s3manager"
	"github.com/go-logr/logr"
	"github.com/kyokomi/emoji"
	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
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
		"cert-manager-tls":                  RegistryDeploymentID,
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
			ui.Note().Msgf("retrying to apply %s", tektonPipelineYamlPath)
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
	// all application namespaces to that Kubernetes can pull application images.
	//
	// TODO: Does it need kubed to be running before this can work?
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

// The equivalent of:
// kubectl get secret -n tekton-staging epinio-registry-tls -o json | jq -r '.["data"]["ca.crt"]' | base64 -d | openssl x509 -hash -noout
// written in golang.
func getRegistryCAHash(ctx context.Context, c *kubernetes.Cluster) (string, error) {
	secret, err := c.Kubectl.CoreV1().Secrets(TektonStagingNamespace).
		Get(ctx, RegistryCertSecret, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	// cert-manager doesn't add the CA for ACME certificates:
	// https://github.com/jetstack/cert-manager/issues/2111
	if _, found := secret.Data["ca.crt"]; !found {
		return "", nil
	}

	hash, err := GenerateHash(secret.Data["ca.crt"])
	if err != nil {
		return "", err
	}

	return hash, nil
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

	// Trust our own CA if an external registry is not used
	if options.GetStringNG("external-registry-url") == "" {
		if err := k.mountCA(ctx, c, tektonTask); err != nil {
			return errors.Wrapf(err, "creating the volume mount for the registry CA")
		}
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

// -----------------------------------------------------------------------------------

func GenerateHash(certRaw []byte) (string, error) {
	cert, err := decodeOneCert(certRaw)
	if err != nil {
		return "", fmt.Errorf("failed to decode certificate\n%w", err)
	}

	hash, err := SubjectNameHash(cert)
	if err != nil {
		return "", fmt.Errorf("failed compute subject name hash for cert\n%w", err)
	}

	name := fmt.Sprintf("%08x", hash)
	return name, nil
}

// -----------------------------------------------------------------------------------
// See gh:paketo-buildpacks/ca-certificates (cacerts/certs.go) for the original code.

func decodeOneCert(raw []byte) (*x509.Certificate, error) {
	block, rest := pem.Decode(raw)
	if block == nil {
		return nil, errors.New("failed find PEM data")
	}
	extra, _ := pem.Decode(rest)
	if extra != nil {
		return nil, errors.New("found multiple PEM blocks, expected exactly one")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certficate\n%w", err)
	}
	return cert, nil
}

// SubjectNameHash is a reimplementation of the X509_subject_name_hash
// in openssl. It computes the SHA-1 of the canonical encoding of the
// certificate's subject name and returns the 32-bit integer
// represented by the first four bytes of the hash using little-endian
// byte order.
func SubjectNameHash(cert *x509.Certificate) (uint32, error) {
	name, err := CanonicalName(cert.RawSubject)
	if err != nil {
		return 0, fmt.Errorf("failed to compute canonical subject name\n%w", err)
	}
	hasher := sha1.New() // nolint:gosec // Required by subject hash specification
	_, err = hasher.Write(name)
	if err != nil {
		return 0, fmt.Errorf("failed to compute sha1sum of canonical subject name\n%w", err)
	}
	sum := hasher.Sum(nil)
	return binary.LittleEndian.Uint32(sum[:4]), nil
}

// canonicalSET holds a of canonicalATVs. Suffix SET ensures it is
// marshaled as a set rather than a sequence by asn1.Marshal.
type canonicalSET []canonicalATV

// canonicalATV is similar to pkix.AttributeTypeAndValue but includes
// tag to ensure all values are marshaled as ASN.1, UTF8String values
type canonicalATV struct {
	Type  asn1.ObjectIdentifier
	Value string `asn1:"utf8"`
}

// CanonicalName accepts a DER encoded subject name and returns a
// "Canonical Encoding" matching that returned by the x509_name_canon
// function in openssl. All string values are transformed with
// CanonicalString and UTF8 encoded and the leading SEQ header is
// removed.
//
// For more information see
// https://stackoverflow.com/questions/34095440/hash-algorithm-for-certificate-crl-directory.
func CanonicalName(name []byte) ([]byte, error) {
	var origSeq pkix.RDNSequence
	_, err := asn1.Unmarshal(name, &origSeq)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subject name\n%w", err)
	}
	var result []byte
	for _, origSet := range origSeq {
		var canonSet canonicalSET
		for _, origATV := range origSet {
			origVal, ok := origATV.Value.(string)
			if !ok {
				return nil, errors.New("got unexpected non-string value")
			}
			canonSet = append(canonSet, canonicalATV{
				Type:  origATV.Type,
				Value: CanonicalString(origVal),
			})
		}
		setBytes, err := asn1.Marshal(canonSet)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal canonical name\n%w", err)
		}
		result = append(result, setBytes...)
	}
	return result, nil
}

// CanonicalString transforms the given string. All leading and
// trailing whitespace is trimmed where whitespace is defined as a
// space, formfeed, tab, newline, carriage return, or vertical tab
// character. Any remaining sequence of one or more consecutive
// whitespace characters in replaced with a single ' '.
//
// This is a reimplementation of the asn1_string_canon in openssl
func CanonicalString(s string) string {
	s = strings.TrimLeft(s, " \f\t\n\v")
	s = strings.TrimRight(s, " \f\t\n\v")
	s = strings.ToLower(s)
	return string(regexp.MustCompile(`[[:space:]]+`).ReplaceAll([]byte(s), []byte(" ")))
}

// storeS3Settings stores the provides S3 settings in a Secret.
func (k Tekton) storeS3Settings(ctx context.Context, cluster *kubernetes.Cluster) error {
	_, err := s3manager.StoreConnectionDetails(ctx, cluster, TektonStagingNamespace, S3ConnectionDetailsSecret, *k.S3ConnectionDetails)

	return err
}

// TODO this workaround is only needed for untrusted certs.
//  Once we can reach Tekton via linkerd, blocked by
//  https://github.com/tektoncd/catalog/issues/757, we can remove the workaround.
func (k Tekton) mountCA(ctx context.Context, c *kubernetes.Cluster, tektonTask *v1beta1.Task) error {
	var caHash string
	var err error

	k.Log.Info(fmt.Sprintf("Checking registry certificates in %s", TektonStagingNamespace))
	if err := k.waitForRegistryCA(ctx, c); err != nil {
		return errors.Wrap(err, "waiting for the registry CA")
	}

	// Add volume and volume mount of registry-certs for local deployment
	// since tekton should trust the registry-certs.
	retryErr := retry.Do(func() error {
		caHash, err = getRegistryCAHash(ctx, c)
		return err
	},
		retry.RetryIf(func(err error) bool {
			return strings.Contains(err.Error(), "failed find PEM data")
		}),
		retry.OnRetry(func(n uint, err error) {
			k.Log.Info(fmt.Sprintf("Retrying fetching of CA hash from registry certificate secret (%d/%d)", n, duration.RetryMax))
		}),
		retry.Delay(5*time.Second),
		retry.Attempts(duration.RetryMax),
	)
	if retryErr != nil {
		return errors.Wrapf(err, "Failed to get registry CA from %s namespace", TektonStagingNamespace)
	}

	if caHash != "" {
		volume := corev1.Volume{
			Name: "registry-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: RegistryCertSecret,
				},
			},
		}
		tektonTask.Spec.Volumes = append(tektonTask.Spec.Volumes, volume)

		volumeMount := corev1.VolumeMount{
			Name:      "registry-certs",
			MountPath: fmt.Sprintf("%s/%s", "/etc/ssl/certs", caHash),
			SubPath:   "ca.crt",
			ReadOnly:  true,
		}
		for stepIndex, step := range tektonTask.Spec.Steps {
			if step.Name == "create" {
				tektonTask.Spec.Steps[stepIndex].VolumeMounts = append(tektonTask.Spec.Steps[stepIndex].VolumeMounts, volumeMount)
				break
			}
		}
	}
	return nil
}

func (k Tekton) waitForRegistryCA(ctx context.Context, cluster *kubernetes.Cluster) error {
	_, err := cluster.WaitForSecret(ctx, TektonStagingNamespace, RegistryCertSecret, duration.ToSecretCopied())
	if err != nil {
		return err
	}

	out, err := helpers.ExecToSuccessWithTimeout(
		func() (string, error) {
			out, err := helpers.Kubectl("get", "secret",
				"--namespace", TektonStagingNamespace, RegistryCertSecret,
				"-o", "jsonpath={.data.tls\\.crt}")
			if err != nil {
				return "", err
			}

			if out == "" {
				return "", errors.New("secret is not filled")
			}
			return out, nil
		}, k.Log, k.Timeout, duration.PollInterval())
	if err != nil {
		return errors.Wrapf(err, "waiting for registry ca failed:\n%s", out)
	}

	return nil
}
