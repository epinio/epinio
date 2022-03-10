// Package configurations encapsulates all the functionality around Epinio configurations
package configurations

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/epinio/epinio/helpers/kubernetes"
	epinioerrors "github.com/epinio/epinio/internal/errors"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type ConfigurationList []*Configuration

// Configuration contains the information needed for Epinio to address a specific configuration.
type Configuration struct {
	SecretName    string
	NamespaceName string
	Configuration string
	Username      string
	kubeClient    *kubernetes.Cluster
}

// Lookup locates a Configuration by namespace and name. It finds the Configuration
// instance by looking for the relevant Secret.
func Lookup(ctx context.Context, kubeClient *kubernetes.Cluster, namespace, configuration string) (*Configuration, error) {
	// TODO 844 inline

	secretName := configurationResourceName(namespace, configuration)

	s, err := kubeClient.GetSecret(ctx, namespace, secretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errors.New("configuration not found")
		}
		return nil, err
	}
	username := s.ObjectMeta.Labels["app.kubernetes.io/created-by"]

	return &Configuration{
		SecretName:    secretName,
		NamespaceName: namespace,
		Configuration: configuration,
		kubeClient:    kubeClient,
		Username:      username,
	}, nil
}

// List returns a ConfigurationList of all available Configurations in the specified namespace. If no namespace is
// specified (empty string) then configurations across all namespaces are returned.
func List(ctx context.Context, cluster *kubernetes.Cluster, namespace string) (ConfigurationList, error) {
	labelSelector := "app.kubernetes.io/name=epinio"

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

		labelSelector = fmt.Sprintf("%s, epinio.suse.org/namespace=%s", labelSelector, namespace)
	}

	secrets, err := cluster.Kubectl.CoreV1().
		Secrets(namespace).List(ctx,
		metav1.ListOptions{
			LabelSelector: labelSelector,
		})

	if err != nil {
		return nil, err
	}

	result := ConfigurationList{}

	for _, s := range secrets.Items {
		configuration := s.ObjectMeta.Labels["epinio.suse.org/configuration"]
		namespace := s.ObjectMeta.Labels["epinio.suse.org/namespace"]
		username := s.ObjectMeta.Labels["app.kubernetes.io/created-by"]

		secretName := s.ObjectMeta.Name

		result = append(result, &Configuration{
			SecretName:    secretName,
			NamespaceName: namespace,
			Configuration: configuration,
			kubeClient:    cluster,
			Username:      username,
		})
	}

	return result, nil
}

// CreateConfiguration creates a new  configuration instance from namespace,
// name, and a map of parameters.
func CreateConfiguration(ctx context.Context, cluster *kubernetes.Cluster, name, namespace, username string,
	data map[string]string) (*Configuration, error) {

	secretName := configurationResourceName(namespace, name)

	_, err := cluster.GetSecret(ctx, namespace, secretName)
	if err == nil {
		return nil, errors.New("Configuration of this name already exists.")
	}

	// Convert from `string -> string` to the `string -> []byte` expected
	// by kube.
	sdata := make(map[string][]byte)
	for k, v := range data {
		sdata[k] = []byte(v)
	}

	err = cluster.CreateLabeledSecret(ctx, namespace, secretName, sdata,
		map[string]string{
			// "epinio.suse.org/configuration-type": "custom",
			"epinio.suse.org/configuration": name,
			"epinio.suse.org/namespace":     namespace,
			"app.kubernetes.io/name":        "epinio",
			"app.kubernetes.io/created-by":  username,
			// "app.kubernetes.io/version":     cmd.Version
			// FIXME: Importing cmd causes cycle
			// FIXME: Move version info to separate package!
		},
	)
	if err != nil {
		return nil, err
	}
	return &Configuration{
		SecretName:    secretName,
		NamespaceName: namespace,
		Configuration: name,
		kubeClient:    cluster,
	}, nil
}

// UpdateConfiguration modifies an existing configuration as per the instructions and writes
// the result back to the resource.
func UpdateConfiguration(ctx context.Context, cluster *kubernetes.Cluster, configuration *Configuration, changes models.ConfigurationUpdateRequest) error {
	secretName := configurationResourceName(configuration.NamespaceName, configuration.Configuration)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		configurationSecret, err := cluster.GetSecret(ctx, configuration.NamespaceName, secretName)
		if err != nil {
			return err
		}

		for _, remove := range changes.Remove {
			delete(configurationSecret.Data, remove)
		}
		for key, value := range changes.Set {
			configurationSecret.Data[key] = []byte(value)
		}

		_, err = cluster.Kubectl.CoreV1().Secrets(configuration.NamespaceName).Update(
			ctx, configurationSecret, metav1.UpdateOptions{})
		return err
	})
}

// ReplaceConfiguration replaces an existing configuration
func ReplaceConfiguration(ctx context.Context, cluster *kubernetes.Cluster, configuration *Configuration, data map[string]string) (bool, error) {
	secretName := configurationResourceName(configuration.NamespaceName, configuration.Configuration)

	configurationSecret, err := cluster.GetSecret(ctx, configuration.NamespaceName, secretName)
	if err != nil {
		return false, err
	}

	oldData := configurationSecret.Data

	configurationSecret.Data = map[string][]byte{}
	for k, v := range data {
		configurationSecret.Data[k] = []byte(v)
	}
	if reflect.DeepEqual(oldData, configurationSecret.Data) {
		return false, nil
	}

	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = cluster.Kubectl.CoreV1().Secrets(configuration.NamespaceName).Update(
			ctx, configurationSecret, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

// Name returns the configuration instance's name
func (s *Configuration) Name() string {
	return s.Configuration
}

// User returns the configuration's username
func (s *Configuration) User() string {
	return s.Username
}

// Namespace returns the configuration instance's namespace
func (s *Configuration) Namespace() string {
	return s.NamespaceName
}

// GetBinding returns the secret representing the instance's binding
// to the application. This is actually the instance's secret itself,
// independent of the application.
func (s *Configuration) GetBinding(ctx context.Context, appName string, _ string) (*corev1.Secret, error) {
	cluster := s.kubeClient
	configurationSecret, err := cluster.GetSecret(ctx, s.NamespaceName, s.SecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errors.New("configuration does not exist")
		}
		return nil, err
	}

	return configurationSecret, nil
}

// Delete destroys the configuration instance, i.e. its underlying secret
// holding the instance's parameters
func (s *Configuration) Delete(ctx context.Context) error {
	return s.kubeClient.DeleteSecret(ctx, s.NamespaceName, s.SecretName)
}

// Details returns the configuration instance's configuration.
// I.e. the parameter data.
func (s *Configuration) Details(ctx context.Context) (map[string]string, error) {
	configurationSecret, err := s.kubeClient.GetSecret(ctx, s.NamespaceName, s.SecretName)
	if err != nil {
		return nil, err
	}

	details := map[string]string{}

	for k, v := range configurationSecret.Data {
		details[k] = string(v)
	}

	return details, nil
}

// configurationResourceName returns a name for a kube configuration resource
// representing the namespace and configuration
func configurationResourceName(namespace, configuration string) string {
	return fmt.Sprintf("configuration.namespace-%s.conf-%s", namespace, configuration)
}
