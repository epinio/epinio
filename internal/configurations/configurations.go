// Package configurations encapsulates all the functionality around Epinio configurations
// A Configuration is essentially a Secret with some Epinio specific labels.
// This allows us to use any Secret as a Configuration as long as someone labels
// it as such. In the future, we will use this to expose secrets created by Service
// helm charts as Configurations (https://github.com/epinio/epinio/issues/1281).
// Since we don't control the name of the produced secret in that case, we will
// need some method to tie a Configuration to a Service. This can be solved with
// some labels on the Configuration or the Service instance resource.
package configurations

import (
	"context"
	"errors"
	"reflect"

	"github.com/epinio/epinio/helpers/kubernetes"
	epinioerrors "github.com/epinio/epinio/internal/errors"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	ConfigurationLabelKey     = "epinio.suse.org/configuration"
	ConfigurationTypeLabelKey = "epinio.suse.org/configuration-type"
)

type ConfigurationList []*Configuration

// Configuration contains the information needed for Epinio to address a specific configuration.
type Configuration struct {
	Name       string
	Namespace  string
	Username   string
	CreatedAt  metav1.Time
	kubeClient *kubernetes.Cluster
}

// Lookup locates a Configuration by namespace and name.
// It finds the Configuration instance by looking for the relevant Secret.
func Lookup(ctx context.Context, kubeClient *kubernetes.Cluster, namespace, configuration string) (*Configuration, error) {
	c := &Configuration{
		Name:       configuration,
		Namespace:  namespace,
		kubeClient: kubeClient,
	}

	s, err := c.GetSecret(ctx)
	if err != nil {
		return nil, err
	}
	c.Username = s.ObjectMeta.Labels["app.kubernetes.io/created-by"]
	c.CreatedAt = s.ObjectMeta.CreationTimestamp

	return c, nil
}

// List returns a ConfigurationList of all available Configurations in the specified namespace. If no namespace is
// specified (empty string) then configurations across all namespaces are returned.
func List(ctx context.Context, cluster *kubernetes.Cluster, namespace string) (ConfigurationList, error) {
	// Verify namespace, if specified
	if namespace != "" {
		exists, err := namespaces.Exists(ctx, cluster, namespace)
		if err != nil {
			return ConfigurationList{}, err
		}
		if !exists {
			return ConfigurationList{}, epinioerrors.NamespaceMissingError{
				Namespace: namespace,
			}
		}
	}

	secretSelector := labels.Set(map[string]string{
		ConfigurationLabelKey: "true",
	}).AsSelector()

	listOpts := metav1.ListOptions{LabelSelector: secretSelector.String()}

	secrets, err := cluster.Kubectl.CoreV1().Secrets(namespace).List(ctx, listOpts)
	if err != nil {
		return nil, err
	}

	result := ConfigurationList{}

	for _, s := range secrets.Items {
		name := s.Name
		namespace := s.Namespace
		username := s.ObjectMeta.Labels["app.kubernetes.io/created-by"]

		result = append(result, &Configuration{
			CreatedAt:  s.ObjectMeta.CreationTimestamp,
			Name:       name,
			Namespace:  namespace,
			Username:   username,
			kubeClient: cluster,
		})
	}

	return result, nil
}

// CreateConfiguration creates a new  configuration instance from namespace,
// name, and a map of parameters.
func CreateConfiguration(ctx context.Context, cluster *kubernetes.Cluster, name, namespace, username string,
	data map[string]string) (*Configuration, error) {

	_, err := cluster.GetSecret(ctx, namespace, name)
	if err == nil {
		return nil, errors.New("a secret for this configuration already exists")
	}

	// Convert from `string -> string` to the `string -> []byte` expected
	// by kube.
	sdata := make(map[string][]byte)
	for k, v := range data {
		sdata[k] = []byte(v)
	}

	labels := map[string]string{
		ConfigurationLabelKey:          "true",
		ConfigurationTypeLabelKey:      "custom",
		"app.kubernetes.io/created-by": username,
		"app.kubernetes.io/name":       "epinio",
		// "app.kubernetes.io/version":     cmd.Version
		// FIXME: Importing cmd causes cycle
		// FIXME: Move version info to separate package!
	}

	err = cluster.CreateLabeledSecret(ctx, namespace, name, sdata, labels)
	if err != nil {
		return nil, err
	}

	return &Configuration{
		Name:       name,
		Namespace:  namespace,
		kubeClient: cluster,
	}, nil
}

// UpdateConfiguration modifies an existing configuration as per the instructions and writes
// the result back to the resource.
func UpdateConfiguration(ctx context.Context, cluster *kubernetes.Cluster, configuration *Configuration, changes models.ConfigurationUpdateRequest) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		secret, err := configuration.GetSecret(ctx)
		if err != nil {
			return err
		}

		for _, remove := range changes.Remove {
			delete(secret.Data, remove)
		}
		for key, value := range changes.Set {
			secret.Data[key] = []byte(value)
		}

		_, err = cluster.Kubectl.CoreV1().Secrets(configuration.Namespace).Update(
			ctx, secret, metav1.UpdateOptions{})
		return err
	})
}

// ReplaceConfiguration replaces an existing configuration
func ReplaceConfiguration(ctx context.Context, cluster *kubernetes.Cluster, configuration *Configuration, data map[string]string) (bool, error) {
	secret, err := configuration.GetSecret(ctx)
	if err != nil {
		return false, err
	}

	oldData := secret.Data

	secret.Data = map[string][]byte{}
	for k, v := range data {
		secret.Data[k] = []byte(v)
	}
	if reflect.DeepEqual(oldData, secret.Data) {
		return false, nil
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = cluster.Kubectl.CoreV1().Secrets(configuration.Namespace).Update(
			ctx, secret, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

// User returns the configuration's username
func (s *Configuration) User() string {
	return s.Username
}

func (s *Configuration) GetSecret(ctx context.Context) (*v1.Secret, error) {
	notFoundError := errors.New("configuration not found")

	secret, err := s.kubeClient.GetSecret(ctx, s.Namespace, s.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, notFoundError
		}
		return nil, err
	}
	if val, ok := secret.Labels[ConfigurationLabelKey]; !ok || val != "true" {
		return nil, notFoundError
	}

	return secret, nil
}

// ForService returns a slice of Secrets matching the given Service.
func ForService(ctx context.Context, kubeClient *kubernetes.Cluster, namespace, name string) ([]v1.Secret, error) {
	secretSelector := labels.Set(map[string]string{
		"app.kubernetes.io/managed-by": "Helm",
		"app.kubernetes.io/instance":   names.ServiceHelmChartName(name, namespace),
		ConfigurationLabelKey:          "true",
		ConfigurationTypeLabelKey:      "service",
	}).AsSelector()

	listOptions := metav1.ListOptions{
		FieldSelector: "type=Opaque",
		LabelSelector: secretSelector.String(),
	}

	secretList, err := kubeClient.Kubectl.CoreV1().Secrets(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	return secretList.Items, nil
}

// LabelServiceSecrets will look for the Opaque secrets released with a service, looking for the
// app.kubernetes.io/instance label, then it will add the Configuration labels to "create" the configurations
func LabelServiceSecrets(ctx context.Context, kubeClient *kubernetes.Cluster, namespace, name string) ([]v1.Secret, error) {
	secretSelector := labels.Set(map[string]string{
		"app.kubernetes.io/managed-by": "Helm",
		"app.kubernetes.io/instance":   names.ServiceHelmChartName(name, namespace),
	}).AsSelector()

	listOptions := metav1.ListOptions{
		FieldSelector: "type=Opaque",
		LabelSelector: secretSelector.String(),
	}

	// Find all user credential secrets
	secretList, err := kubeClient.Kubectl.CoreV1().Secrets(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	for _, secret := range secretList.Items {
		sec := secret

		// set labels without override the old ones
		sec.GetLabels()[ConfigurationLabelKey] = "true"
		sec.GetLabels()[ConfigurationTypeLabelKey] = "service"

		_, err = kubeClient.Kubectl.CoreV1().Secrets(namespace).Update(ctx, &sec, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	}

	return secretList.Items, nil
}

// Delete destroys the configuration instance, i.e. its underlying secret
// holding the instance's parameters
func (s *Configuration) Delete(ctx context.Context) error {
	return s.kubeClient.DeleteSecret(ctx, s.Namespace, s.Name)
}

// Details returns the configuration instance's configuration.
// I.e. the parameter data.
func (s *Configuration) Details(ctx context.Context) (map[string]string, error) {
	secret, err := s.GetSecret(ctx)
	if err != nil {
		return nil, err
	}

	details := map[string]string{}

	for k, v := range secret.Data {
		details[k] = string(v)
	}

	return details, nil
}
