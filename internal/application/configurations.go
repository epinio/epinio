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

package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type NameSet map[string]struct{}

// BoundApps is an extension of BoundAppsNames after it, to retrieve a map of configurations to
// the full data of the applications bound to them. It uses BoundAppsNames internally to
// quickly determine the applications to fetch.
func BoundApps(ctx context.Context, cluster *kubernetes.Cluster, namespace string) (map[string]models.AppList, error) {

	result := map[string]models.AppList{}

	bindings, err := BoundAppsNames(ctx, cluster, namespace)
	if err != nil {
		return result, err
	}

	// Internal map of fetched applications.
	fetched := map[string]*models.App{}

	for key, appNames := range bindings {
		configurationName, namespace := DecodeConfigurationKey(key)
		for _, appName := range appNames {
			app, ok := fetched[appName]
			if !ok {
				meta := models.NewAppRef(appName, namespace)
				app := meta.App()
				err := fetch(ctx, cluster, app)
				if err != nil {
					// Ignoring the error. Assumption is that
					// the app got deleted as this function is
					// collecting its information.
					break
				}
				fetched[appName] = app
			}
			result[configurationName] = append(result[configurationName], *app)
		}
	}

	return result, nil
}

// BoundAppsNamesFor is a specialization of BoundAppsNames after it, to retrieve the names
// of the applications bound to a single configuration, specified by name.
func BoundAppsNamesFor(ctx context.Context, cluster *kubernetes.Cluster, namespace, configurationName string) ([]string, error) {
	result := []string{}

	// locate configuration bindings managed by epinio applications
	selector := EpinioApplicationAreaLabel + "=configuration"
	selector += ",app.kubernetes.io/component=application"
	selector += ",app.kubernetes.io/managed-by=epinio"

	appBindings, err := cluster.Kubectl.CoreV1().Secrets(namespace).List(ctx,
		metav1.ListOptions{
			LabelSelector: selector,
		})
	if err != nil {
		return result, err
	}

	// Instead of building a full inverted map from configuration names to app names here we
	// filter on the configuration name to generate just that slice of app names.

	for _, binding := range appBindings.Items {
		for boundConfigurationName := range binding.Data {
			if boundConfigurationName == configurationName {
				appName := binding.ObjectMeta.Labels["app.kubernetes.io/name"]
				result = append(result, appName)
				break
			}
		}
	}

	return result, nil
}

// BoundAppsNames returns a map from the names of configurations in the specified namespace, to the
// names of the applications they are bound to. The keys of the map are always a combination of
// namespace name and configuration name, to distinguish same-named configurations in different
// namespaces (See `ConfigurationKey` below). The application names never contain namespace
// information, as they are always in the same namespace as the configuration referencing them.
func BoundAppsNames(ctx context.Context, cluster *kubernetes.Cluster, namespace string) (map[ConfigurationKey][]string, error) {

	result := map[ConfigurationKey][]string{}

	// locate configuration bindings managed by epinio applications.
	selector := EpinioApplicationAreaLabel + "=configuration"
	selector += ",app.kubernetes.io/component=application"
	selector += ",app.kubernetes.io/managed-by=epinio"

	appBindings, err := cluster.Kubectl.CoreV1().Secrets(namespace).List(ctx,
		metav1.ListOptions{
			LabelSelector: selector,
		})
	if err != nil {
		return result, err
	}

	for _, binding := range appBindings.Items {
		appName := binding.ObjectMeta.Labels["app.kubernetes.io/name"]
		namespace := binding.ObjectMeta.Labels["app.kubernetes.io/part-of"]

		for configurationName := range binding.Data {
			key := EncodeConfigurationKey(configurationName, namespace)
			result[key] = append(result[key], appName)
		}
	}

	return result, nil
}

// ConfigurationKey is a type used to create a unique key for an app/namespace
type ConfigurationKey string

// EncodeConfigurationKey constructs a single key string from configuration and namespace names, for the
// `configurationsToApps` map, when used for configurations and apps across all namespaces. It uses
// ASCII NUL (\000) as the separator character. NUL is forbidden to occur in the names
// themselves. This should make it impossible to construct two different pairs of
// configuration/namespace names which map to the same key.
func EncodeConfigurationKey(name, namespace string) ConfigurationKey {
	return ConfigurationKey(fmt.Sprintf("%s\000%s", name, namespace))
}

// DecodeConfigurationKey splits the given key back into name and namespace.
// The name is the first result, the namespace the second.
func DecodeConfigurationKey(key ConfigurationKey) (string, string) {
	parts := strings.Split(string(key), "\000")
	return parts[0], parts[1]
}

// BoundConfigurationNameSet returns the configuration names for the configurations bound to the
// application by a user, as a map/set.
func BoundConfigurationNameSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (NameSet, error) {
	configSecret, err := configLoad(ctx, cluster, appRef)
	if err != nil {
		return nil, err
	}

	result := NameSet{}
	for name := range configSecret.Data {
		result[name] = struct{}{}
	}

	return result, nil
}

// BoundConfigurationNames returns the configuration names for the configurations bound to the
// application by a user, as a slice. Ordered by name.
func BoundConfigurationNames(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) ([]string, error) {
	configSecret, err := configLoad(ctx, cluster, appRef)
	if err != nil {
		return nil, err
	}

	return BoundConfigurationNamesFromSecret(configSecret), nil
}

// BoundConfigurationNamesFromSecret is the core of BoundConfigurationNames, extracting the set of
// configuration names from the secret containing them.
func BoundConfigurationNamesFromSecret(configSecret *v1.Secret) []string {
	result := []string{}
	for name := range configSecret.Data {
		result = append(result, name)
	}

	// Normalize to lexicographic order.
	sort.Strings(result)

	return result
}

// BoundConfigurationsSet replaces or adds the specified configuration names to the named application.
// When the function returns the configuration set will be extended.
// Adding a known configuration is a no-op.
func BoundConfigurationsSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, configurationNames []string, replace bool) error {
	return configUpdate(ctx, cluster, appRef, func(configSecret *v1.Secret) {
		// Replacement is adding to a clear structure
		if replace {
			configSecret.Data = make(map[string][]byte)
		}
		for _, configurationName := range configurationNames {
			configSecret.Data[configurationName] = nil
		}
	})
}

// BoundConfigurationsUnset removes the specified configuration name from the named application.
// When the function returns the configuration set will be shrunk.
// Removing an unknown configuration is a no-op.
func BoundConfigurationsUnset(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, configurationNames []string) error {
	return configUpdate(ctx, cluster, appRef, func(configSecret *v1.Secret) {
		for _, c := range configurationNames {
			delete(configSecret.Data, c)
		}
	})
}

// configUpdate is a helper for the public functions. It encapsulates the read/modify/write cycle
// necessary to update the application's kube resource holding the application's configuration
// names.
func configUpdate(ctx context.Context, cluster *kubernetes.Cluster,
	appRef models.AppRef, modifyBoundConfigurations func(*v1.Secret)) error {

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		configSecret, err := configLoad(ctx, cluster, appRef)
		if err != nil {
			return err
		}

		if configSecret.Data == nil {
			configSecret.Data = make(map[string][]byte)
		}

		modifyBoundConfigurations(configSecret)

		_, err = cluster.Kubectl.CoreV1().Secrets(appRef.Namespace).Update(
			ctx, configSecret, metav1.UpdateOptions{})

		return err
	})
}

// configLoad locates and returns the kube secret storing the referenced application's bound
// configurations' names. If necessary it creates that secret.
func configLoad(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*v1.Secret, error) {
	secretName := appRef.MakeConfigurationSecretName()
	return loadOrCreateSecret(ctx, cluster, appRef, secretName, "configuration")
}
