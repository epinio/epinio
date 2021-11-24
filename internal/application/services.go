package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type NameSet map[string]struct{}

// BoundApps is an extension of BoundAppsNames after it, to retrieve a map of services to
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
		serviceName, namespace := DecodeServiceKey(key)
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
			result[serviceName] = append(result[serviceName], *app)
		}
	}

	return result, nil
}

// BoundAppsNamesFor is a specialization of BoundAppsNames after it, to retrieve the names
// of the applications bound to a single service, specified by name.

func BoundAppsNamesFor(ctx context.Context, cluster *kubernetes.Cluster, namespace, serviceName string) ([]string, error) {
	result := []string{}

	// locate service bindings managed by epinio applications
	selector := EpinioApplicationAreaLabel + "=service"
	selector += ",app.kubernetes.io/component=application"
	selector += ",app.kubernetes.io/managed-by=epinio"

	appBindings, err := cluster.Kubectl.CoreV1().Secrets(namespace).List(ctx,
		metav1.ListOptions{
			LabelSelector: selector,
		})
	if err != nil {
		return result, err
	}

	// Instead of building a full inverted map from service names to app names here we
	// filter on the service name to generate just that slice of app names.

	for _, binding := range appBindings.Items {
		for boundServiceName := range binding.Data {
			if boundServiceName == serviceName {
				appName := binding.ObjectMeta.Labels["app.kubernetes.io/name"]
				result = append(result, appName)
				break
			}
		}
	}

	return result, nil
}

// BoundAppsNames returns a map from the service names of services in the specified
// namespace, to the names of the applications they are bound to. The keys of the map are
// always a combination of namespace name and service name, to distinguish same-named
// services in different namespaces (See `ServiceKey` below). The application names never
// contain namespace information, as they are always in the same namespace as the service
// referencing them.

func BoundAppsNames(ctx context.Context, cluster *kubernetes.Cluster, namespace string) (map[string][]string, error) {

	result := map[string][]string{}

	// locate service bindings managed by epinio applications.
	selector := EpinioApplicationAreaLabel + "=service"
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

		for serviceName := range binding.Data {
			key := ServiceKey(serviceName, namespace)
			result[key] = append(result[key], appName)
		}
	}

	return result, nil
}

// ServiceKey constructs a single key string from service and namespace names, for the
// `servicesToApps` map, when used for services and apps across all namespaces. It uses
// ASCII NUL (\000) as the separator character. NUL is forbidden to occur in the names
// themselves. This should make it impossible to construct two different pairs of
// service/namespace names which map to the same key.
func ServiceKey(name, namespace string) string {
	return fmt.Sprintf("%s\000%s", name, namespace)
}

// DecodeServiceKey splits the given key back into name and namespace.
// The name is the first result, the namespace the second.
func DecodeServiceKey(key string) (string, string) {
	parts := strings.Split(key, "\0000")
	return parts[0], parts[1]
}

// BoundServices returns the set of services bound to the application. Ordered by name.
func BoundServices(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (services.ServiceList, error) {

	names, err := BoundServiceNames(ctx, cluster, appRef)
	if err != nil {
		return nil, err
	}

	var bound = services.ServiceList{}

	for _, name := range names {
		service, err := services.Lookup(ctx, cluster, appRef.Namespace, name)
		if err != nil {
			return nil, err
		}
		bound = append(bound, service)
	}

	return bound, nil
}

// BoundServiceNameSet returns the service names for the services bound to the application by a user, as a map/set
func BoundServiceNameSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (NameSet, error) {
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

// BoundServiceNames returns the service names for the services bound to the application by a user, as a slice. Ordered by name.
func BoundServiceNames(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) ([]string, error) {
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

// BoundServicesSet replaces or adds the specified service names to the named application.
// When the function returns the service set will be extended.
// Adding a known service is a no-op.
func BoundServicesSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, serviceNames []string, replace bool) error {
	return svcUpdate(ctx, cluster, appRef, func(svcSecret *v1.Secret) {
		// Replacement is adding to a clear structure
		if replace {
			svcSecret.Data = make(map[string][]byte)
		}
		for _, serviceName := range serviceNames {
			svcSecret.Data[serviceName] = nil
		}
	})
}

// BoundServicesUnset removes the specified service name from the named application.
// When the function returns the service set will be shrunk.
// Removing an unknown service is a no-op.
func BoundServicesUnset(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, serviceName string) error {
	return svcUpdate(ctx, cluster, appRef, func(svcSecret *v1.Secret) {
		delete(svcSecret.Data, serviceName)
	})
}

// svcUpdate is a helper for the public functions. It encapsulates the read/modify/write cycle
// necessary to update the application's kube resource holding the application's service names.
func svcUpdate(ctx context.Context, cluster *kubernetes.Cluster,
	appRef models.AppRef, modifyBoundServices func(*v1.Secret)) error {

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		svcSecret, err := svcLoad(ctx, cluster, appRef)
		if err != nil {
			return err
		}

		if svcSecret.Data == nil {
			svcSecret.Data = make(map[string][]byte)
		}

		modifyBoundServices(svcSecret)

		_, err = cluster.Kubectl.CoreV1().Secrets(appRef.Namespace).Update(
			ctx, svcSecret, metav1.UpdateOptions{})

		return err
	})
}

// svcLoad locates and returns the kube secret storing the referenced application's bound services'
// names. If necessary it creates that secret.
func svcLoad(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*v1.Secret, error) {
	secretName := appRef.MakeServiceSecretName()

	svcSecret, err := cluster.GetSecret(ctx, appRef.Namespace, secretName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}

		// Error is `Not Found`. Create the secret.

		app, err := Get(ctx, cluster, appRef)
		if err != nil {
			// Should not happen. The application was validated to exist already somewhere
			// by this function's callers.
			return nil, err
		}

		owner := metav1.OwnerReference{
			APIVersion: app.GetAPIVersion(),
			Kind:       app.GetKind(),
			Name:       app.GetName(),
			UID:        app.GetUID(),
		}

		svcSecret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: appRef.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					owner,
				},
				Labels: map[string]string{
					"app.kubernetes.io/name":       appRef.Name,
					"app.kubernetes.io/part-of":    appRef.Namespace,
					"app.kubernetes.io/managed-by": "epinio",
					"app.kubernetes.io/component":  "application",
					EpinioApplicationAreaLabel:     "service",
				},
			},
		}
		err = cluster.CreateSecret(ctx, appRef.Namespace, *svcSecret)

		if err != nil {
			return nil, err
		}
	}

	return svcSecret, nil
}
