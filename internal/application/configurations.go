package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/configurations"
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

// BoundAppsNames returns a map from the configuration names of configurations in the specified
// namespace, to the names of the applications they are bound to. The keys of the map are
// always a combination of namespace name and configuration name, to distinguish same-named
// configurations in different namespaces (See `ConfigurationKey` below). The application names never
// contain namespace information, as they are always in the same namespace as the configuration
// referencing them.

func BoundAppsNames(ctx context.Context, cluster *kubernetes.Cluster, namespace string) (map[string][]string, error) {

	result := map[string][]string{}

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
			key := ConfigurationKey(configurationName, namespace)
			result[key] = append(result[key], appName)
		}
	}

	return result, nil
}

// ConfigurationKey constructs a single key string from configuration and namespace names, for the
// `configurationsToApps` map, when used for configurations and apps across all namespaces. It uses
// ASCII NUL (\000) as the separator character. NUL is forbidden to occur in the names
// themselves. This should make it impossible to construct two different pairs of
// configuration/namespace names which map to the same key.
func ConfigurationKey(name, namespace string) string {
	return fmt.Sprintf("%s\000%s", name, namespace)
}

// DecodeConfigurationKey splits the given key back into name and namespace.
// The name is the first result, the namespace the second.
func DecodeConfigurationKey(key string) (string, string) {
	parts := strings.Split(key, "\0000")
	return parts[0], parts[1]
}

// BoundConfigurations returns the set of configurations bound to the application. Ordered by name.
func BoundConfigurations(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (configurations.ConfigurationList, error) {

	names, err := BoundConfigurationNames(ctx, cluster, appRef)
	if err != nil {
		return nil, err
	}

	var bound = configurations.ConfigurationList{}

	for _, name := range names {
		configuration, err := configurations.Lookup(ctx, cluster, appRef.Namespace, name)
		if err != nil {
			return nil, err
		}
		bound = append(bound, configuration)
	}

	return bound, nil
}

// BoundConfigurationNameSet returns the configuration names for the configurations bound to the application by a user, as a map/set
func BoundConfigurationNameSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (NameSet, error) {
	svcSecret, err := svcLoad(ctx, cluster, appRef)
	if err != nil {
		return nil, err
	}

	result := NameSet{}
	for name := range svcSecret.Data {
		result[name] = struct{}{}
	}

	return result, nil
}

// BoundConfigurationNames returns the configuration names for the configurations bound to the application by a user, as a slice. Ordered by name.
func BoundConfigurationNames(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) ([]string, error) {
	svcSecret, err := svcLoad(ctx, cluster, appRef)
	if err != nil {
		return nil, err
	}

	result := []string{}
	for name := range svcSecret.Data {
		result = append(result, name)
	}

	// Normalize to lexicographic order.
	sort.Strings(result)

	return result, nil
}

// BoundConfigurationsSet replaces or adds the specified configuration names to the named application.
// When the function returns the configuration set will be extended.
// Adding a known configuration is a no-op.
func BoundConfigurationsSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, configurationNames []string, replace bool) error {
	return svcUpdate(ctx, cluster, appRef, func(svcSecret *v1.Secret) {
		// Replacement is adding to a clear structure
		if replace {
			svcSecret.Data = make(map[string][]byte)
		}
		for _, configurationName := range configurationNames {
			svcSecret.Data[configurationName] = nil
		}
	})
}

// BoundConfigurationsUnset removes the specified configuration name from the named application.
// When the function returns the configuration set will be shrunk.
// Removing an unknown configuration is a no-op.
func BoundConfigurationsUnset(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, configurationName string) error {
	return svcUpdate(ctx, cluster, appRef, func(svcSecret *v1.Secret) {
		delete(svcSecret.Data, configurationName)
	})
}

// svcUpdate is a helper for the public functions. It encapsulates the read/modify/write cycle
// necessary to update the application's kube resource holding the application's configuration names.
func svcUpdate(ctx context.Context, cluster *kubernetes.Cluster,
	appRef models.AppRef, modifyBoundConfigurations func(*v1.Secret)) error {

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		svcSecret, err := svcLoad(ctx, cluster, appRef)
		if err != nil {
			return err
		}

		if svcSecret.Data == nil {
			svcSecret.Data = make(map[string][]byte)
		}

		modifyBoundConfigurations(svcSecret)

		_, err = cluster.Kubectl.CoreV1().Secrets(appRef.Namespace).Update(
			ctx, svcSecret, metav1.UpdateOptions{})

		return err
	})
}

// svcLoad locates and returns the kube secret storing the referenced application's bound configurations'
// names. If necessary it creates that secret.
func svcLoad(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*v1.Secret, error) {
	secretName := appRef.MakeConfigurationSecretName()
	return loadOrCreateSecret(ctx, cluster, appRef, secretName, "configuration")
}
