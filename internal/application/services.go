package application

import (
	"context"
	"sort"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type NameSet map[string]struct{}

// BoundServices returns the set of services bound to the application. Ordered by name.
func BoundServices(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (services.ServiceList, error) {

	names, err := BoundServiceNames(ctx, cluster, appRef)
	if err != nil {
		return nil, err
	}

	var bound = services.ServiceList{}

	for _, name := range names {
		service, err := services.Lookup(ctx, cluster, appRef.Org, name)
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

		_, err = cluster.Kubectl.CoreV1().Secrets(appRef.Org).Update(
			ctx, svcSecret, metav1.UpdateOptions{})

		return err
	})
}

// svcLoad locates and returns the kube secret storing the referenced application's bound services'
// names. If necessary it creates that secret.
func svcLoad(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*v1.Secret, error) {
	secretName := appRef.MakeServiceSecretName()

	svcSecret, err := cluster.GetSecret(ctx, appRef.Org, secretName)
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
				Namespace: appRef.Org,
				OwnerReferences: []metav1.OwnerReference{
					owner,
				},
			},
		}
		err = cluster.CreateSecret(ctx, appRef.Org, *svcSecret)

		if err != nil {
			return nil, err
		}

		err = cluster.LabelSecret(ctx, appRef.Org, secretName, map[string]string{
			"app.kubernetes.io/name":       appRef.Name,
			"app.kubernetes.io/part-of":    appRef.Org,
			"app.kubernetes.io/managed-by": "epinio",
			"app.kubernetes.io/component":  "application",
		})

		if err != nil {
			return nil, err
		}
	}

	return svcSecret, nil
}
