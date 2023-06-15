// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	ConfigurationLabelKey       = "epinio.io/configuration"
	ConfigurationTypeLabelKey   = "epinio.io/configuration-type"
	ConfigurationOriginLabelKey = "epinio.io/configuration-origin"
)

type ConfigurationList []*Configuration

// Configuration contains the information needed for Epinio to address a specific configuration.
type Configuration struct {
	Name       string
	namespace  string
	Username   string
	Type       string
	Origin     string
	CreatedAt  metav1.Time
	kubeClient *kubernetes.Cluster
}

// Lookup locates a Configuration by namespace and name.
// It finds the Configuration instance by looking for the relevant Secret.
func Lookup(ctx context.Context, kubeClient *kubernetes.Cluster, namespace, configuration string) (*Configuration, error) {
	c := &Configuration{
		Name:       configuration,
		namespace:  namespace,
		kubeClient: kubeClient,
	}

	s, err := c.GetSecret(ctx)
	if err != nil {
		return nil, err
	}

	c.Username = s.ObjectMeta.Annotations[models.EpinioCreatedByAnnotation]
	c.Type = s.ObjectMeta.Labels["epinio.io/configuration-type"]
	c.Origin = s.ObjectMeta.Labels["epinio.io/configuration-origin"]
	c.CreatedAt = s.ObjectMeta.CreationTimestamp

	return c, nil
}

// List returns a ConfigurationList of all available Configurations in the specified namespace. If no namespace is
// specified (empty string) then configurations across all namespaces are returned.
func List(ctx context.Context, cluster *kubernetes.Cluster, namespace string) (ConfigurationList, error) {
	// Verify namespace, if specified
	if namespace != "" {
		_, err := namespaces.Exists(ctx, cluster, namespace)
		if err != nil {
			return ConfigurationList{}, err
		}
		// if !exists - Is handled by `NamespaceMiddleware`.
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

	for _, c := range secrets.Items {
		username := c.ObjectMeta.Annotations[models.EpinioCreatedByAnnotation]
		ctype := c.ObjectMeta.Labels["epinio.io/configuration-type"]
		origin := c.ObjectMeta.Labels["epinio.io/configuration-origin"]

		result = append(result, &Configuration{
			CreatedAt:  c.ObjectMeta.CreationTimestamp,
			Name:       c.Name,
			namespace:  c.Namespace,
			Username:   username,
			kubeClient: cluster,
			Type:       ctype,
			Origin:     origin,
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
		ConfigurationLabelKey:     "true",
		ConfigurationTypeLabelKey: "custom",
		"app.kubernetes.io/name":  "epinio",
		// "app.kubernetes.io/version":     cmd.Version
		// FIXME: Importing cmd causes cycle
		// FIXME: Move version info to separate package!
	}

	annotations := map[string]string{
		models.EpinioCreatedByAnnotation: username,
	}

	err = cluster.CreateLabeledSecret(ctx, namespace, name, sdata, labels, annotations)
	if err != nil {
		return nil, err
	}

	return &Configuration{
		Name:       name,
		namespace:  namespace,
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

		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}

		for _, remove := range changes.Remove {
			delete(secret.Data, remove)
		}
		for key, value := range changes.Set {
			secret.Data[key] = []byte(value)
		}

		_, err = cluster.Kubectl.CoreV1().Secrets(configuration.Namespace()).Update(
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
		_, err = cluster.Kubectl.CoreV1().Secrets(configuration.Namespace()).Update(
			ctx, secret, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

// User returns the configuration's username
func (c *Configuration) User() string {
	return c.Username
}

// Namespace returns the configuration's namespace
func (c *Configuration) Namespace() string {
	return c.namespace
}

func (c *Configuration) GetSecret(ctx context.Context) (*v1.Secret, error) {
	notFoundError := errors.New("configuration not found")

	secret, err := c.kubeClient.GetSecret(ctx, c.Namespace(), c.Name)
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

// ForService returns a slice of configuration secrets matching the given Service.
func ForService(ctx context.Context, kubeClient *kubernetes.Cluster, service *models.Service) ([]v1.Secret, error) {
	// The difference to `ForServiceUnlabeled` is here. We are explicitly looking for the labels
	// attached to the secrets by `LabelServiceSecrets` as we want only proper configurations.
	secretSelector := labels.Set(map[string]string{
		ConfigurationLabelKey:     "true",
		ConfigurationTypeLabelKey: "service",
	}).AsSelector()

	return forService(ctx, kubeClient, service, secretSelector)
}

// ForServiceUnlabeled returns a slice of unlabeled secrets matching the given Service
func ForServiceUnlabeled(ctx context.Context, kubeClient *kubernetes.Cluster, service *models.Service) ([]v1.Secret, error) {
	// The difference to `ForService` is here. Not looking for the labels attached to the
	// secrets by `LabelServiceSecrets` to turn them into configurations.
	return forService(ctx, kubeClient, service, labels.NewSelector())
}

func forService(ctx context.Context, kubeClient *kubernetes.Cluster, service *models.Service, secretSelector labels.Selector) ([]v1.Secret, error) {
	// Note how we search here for both regular and old-style secrets in one set-based selector
	// and call to come. No need to perform two requests and merge the results.
	//
	// COMPATIBILITY SUPPORT for services from before https://github.com/epinio/epinio/issues/1704 fix
	// Look for secrets referencing a (helm controller)-based service.

	multiLabel, err := labels.NewRequirement(
		"app.kubernetes.io/instance",
		selection.In,
		[]string{
			names.ServiceReleaseName(service.Meta.Name),
			names.ServiceHelmChartName(service.Meta.Name, service.Meta.Namespace),
		},
	)
	if err != nil {
		return nil, err
	}

	secretSelector = secretSelector.Add(*multiLabel)

	listOptions := metav1.ListOptions{
		LabelSelector: secretSelector.String(),
	}

	secretList, err := kubeClient.Kubectl.CoreV1().Secrets(service.Meta.Namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	filteredSecrets := filterSecretsByType(secretList.Items, service.SecretTypes)

	return filteredSecrets, nil
}

// LabelServiceSecrets will look for the Opaque secrets released with a service, looking for the
// app.kubernetes.io/instance label, then it will add the Configuration labels to "create" the configurations
func LabelServiceSecrets(ctx context.Context, kubeClient *kubernetes.Cluster, service *models.Service) ([]v1.Secret, error) {
	var filteredSecrets []v1.Secret

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Simplification - Get the secrets to handle via the helper above.
		filteredSecretsLocal, err := ForServiceUnlabeled(ctx, kubeClient, service)
		if err != nil {
			return err
		}

		for _, secret := range filteredSecretsLocal {
			sec := secret

			// set labels without overriding the old ones
			sec.GetLabels()[ConfigurationLabelKey] = "true"
			sec.GetLabels()[ConfigurationTypeLabelKey] = "service"
			sec.GetLabels()[ConfigurationOriginLabelKey] = service.Meta.Name

			_, err = kubeClient.Kubectl.CoreV1().Secrets(service.Meta.Namespace).Update(ctx, &sec, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}

		filteredSecrets = filteredSecretsLocal
		return nil
	})
	if err != nil {
		return nil, err
	}

	return filteredSecrets, nil
}

// filterSecretsByType will return a filtered slice of the provided secrets, with the specified secretTypes.
// It's not possible to use the `in` operator with the FieldSelector during the query
// so we need to filter them manually.
// Ref: https://github.com/kubernetes/kubernetes/issues/32946
func filterSecretsByType(secrets []v1.Secret, secretTypes []string) []v1.Secret {
	secretTypesMap := make(map[string]struct{})
	if len(secretTypes) == 0 {
		secretTypesMap["Opaque"] = struct{}{}
	}

	for _, secretType := range secretTypes {
		secretTypesMap[secretType] = struct{}{}
	}

	filteredSecrets := []v1.Secret{}
	for _, secret := range secrets {
		if _, found := secretTypesMap[string(secret.Type)]; !found {
			continue
		}
		filteredSecrets = append(filteredSecrets, secret)
	}

	return filteredSecrets
}

// Delete destroys the configuration instance, i.e. its underlying secret
// holding the instance's parameters
func (c *Configuration) Delete(ctx context.Context) error {
	return c.kubeClient.DeleteSecret(ctx, c.Namespace(), c.Name)
}

// Details returns the configuration instance's configuration.
// I.e. the parameter data.
func (c *Configuration) Details(ctx context.Context) (map[string]string, error) {
	secret, err := c.GetSecret(ctx)
	if err != nil {
		return nil, err
	}

	details := map[string]string{}

	for k, v := range secret.Data {
		details[k] = string(v)
	}

	return details, nil
}
