// Package registry implements the various functions needed to store and retrieve
// images from a container registry.
package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	parser "github.com/novln/docker-parser"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RegistrySecretNamespaceAnnotationKey = "epinio.suse.org/registry-namespace"
	KubedNamespaceSelector               = "kubed-sync=registry-creds"
	CredentialsSecretName                = "registry-creds"
)

type RegistryCredentials struct {
	URL      string
	Username string
	Password string
}

type ContainerRegistryAuth struct {
	Auth     string `json:"auth"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type DockerConfigJSON struct {
	Auths map[string]ContainerRegistryAuth `json:"auths"`
}

type ConnectionDetails struct {
	*DockerConfigJSON
	Namespace string
}

func NewDockerConfigJSON(registries []RegistryCredentials) (*DockerConfigJSON, error) {
	result := DockerConfigJSON{Auths: map[string]ContainerRegistryAuth{}}

	for _, r := range registries {
		if r.URL == "" {
			return nil, errors.New("url must be specified")
		}
		auth := ContainerRegistryAuth{
			Auth:     base64.StdEncoding.EncodeToString([]byte(r.Username + ":" + r.Password)),
			Username: r.Username,
			Password: r.Password,
		}

		result.Auths[r.URL] = auth
	}

	return &result, nil
}

func NewConnectionDetails(config *DockerConfigJSON, namespace string) *ConnectionDetails {
	return &ConnectionDetails{
		DockerConfigJSON: config,
		Namespace:        namespace,
	}
}

// PublicRegistryURL returns the public registry URL from the dockerconfigjson
// object. Assumes there is only one non-local registry in the config. If there
// are more, it will just return the first one found (no guaranteed order since
// there should only be one)
func (d *ConnectionDetails) PublicRegistryURL() (string, error) {
	r, err := regexp.Compile(`127\.0\.0\.1`)
	if err != nil {
		return "", err
	}
	for url := range d.DockerConfigJSON.Auths {
		if !r.MatchString(url) {
			return url, nil
		}
	}

	return "", nil
}

// PrivateRegistryURL return the internal (localhost) registry URL. That
// url can be used by Kubernetes to pull images only when the internal registry
// is used and exposed over NodePort. This method will return an empty string
// if a locahost URL doesn't exist in the config.
func (d *ConnectionDetails) PrivateRegistryURL() (string, error) {
	r, err := regexp.Compile(`127\.0\.0\.1`)
	if err != nil {
		return "", err
	}
	for url := range d.DockerConfigJSON.Auths {
		if r.MatchString(url) {
			return url, nil
		}
	}

	return "", nil
}

// ReplaceWithInternalRegistry replaces the registry part of the given container
// imageURL with the internal (localhost) URL of the registry when:
// - the imageURL is on the Epinio registry (could be deploying from another
//   registry, with the --container-image-url flag)
// - there is a localhost URL defined on the ConnectionDetails (if we are using
//   an external Epinio registry, there is no need to replace anything and there
//   no localhost URL defined either.
func (d *ConnectionDetails) ReplaceWithInternalRegistry(imageURL string) (string, error) {
	privateURL, err := d.PrivateRegistryURL()
	if err != nil {
		return imageURL, err
	}
	if privateURL == "" {
		return imageURL, nil // no-op
	}

	publicURL, err := d.PublicRegistryURL()
	if err != nil {
		return imageURL, err
	}

	imageRegistryURL, _, err := ExtractImageParts(imageURL)
	if err != nil {
		return imageURL, err
	}

	if imageRegistryURL == publicURL {
		return strings.Replace(imageURL, imageRegistryURL, privateURL, -1), nil
	}

	return imageURL, nil
}

// ExtractImageParts accepts a container image URL and returns the registry
// and the image parts.
func ExtractImageParts(imageURL string) (string, string, error) {
	ref, err := parser.Parse(imageURL)
	if err != nil {
		return "", "", err
	}

	return ref.Registry(), ref.Name(), nil
}

// Validate makes sure the provided settings are valid
// The user should provide all the mandatory settings or no settings at all.
func Validate(url, namespace, username, password string) error {
	optionalSet := username != "" || password != "" || namespace != ""

	// If only optional fields are set
	if url == "" && optionalSet {
		return errors.New("do not specify options if using the internal container registry")
	}

	// Either all empty or at least the URL is set
	return nil
}

// GetConnectionDetails retrieves registry connection details from a Kubernetes secret.
func GetConnectionDetails(ctx context.Context, cluster *kubernetes.Cluster, secretNamespace, secretName string) (*ConnectionDetails, error) {
	details := ConnectionDetails{}
	secret, err := cluster.GetSecret(ctx, secretNamespace, secretName)
	if err != nil {
		return nil, err
	}

	var dockerconfigjson DockerConfigJSON
	err = json.Unmarshal(secret.Data[".dockerconfigjson"], &dockerconfigjson)
	if err != nil {
		return nil, err
	}

	details.DockerConfigJSON = &dockerconfigjson
	details.Namespace = secret.ObjectMeta.Annotations[RegistrySecretNamespaceAnnotationKey]

	return &details, nil
}

// Store stores the connection details in a secret.
// The registry namespace (or org) is stored in an annotation (because Kubernetes expects
// the secret in a a specific format). It is used to contruct the full url to
// an application image in the form: registryURL/registryNamespace/appImage
func (d *ConnectionDetails) Store(ctx context.Context, cluster *kubernetes.Cluster, secretNamespace, secretName string) (*corev1.Secret, error) {
	dockerconfigjson, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}

	createdSecret, err := cluster.Kubectl.CoreV1().Secrets(secretNamespace).Create(ctx,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
				Annotations: map[string]string{
					RegistrySecretNamespaceAnnotationKey: d.Namespace,
					"kubed.appscode.com/sync":            KubedNamespaceSelector,
				},
			},
			StringData: map[string]string{
				".dockerconfigjson": string(dockerconfigjson),
			},
			Type: "kubernetes.io/dockerconfigjson",
		}, metav1.CreateOptions{})

	if err != nil {
		return nil, err
	}

	return createdSecret, nil
}
