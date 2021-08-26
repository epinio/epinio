package application

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// Environment returns the environment variables and their values which set on the named application by users
func Environment(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (models.EnvVariableList, error) {
	evSecret, err := envLoad(ctx, cluster, appRef)
	if err != nil {
		return nil, err
	}

	result := models.EnvVariableList{}
	for name, value := range evSecret.Data {
		result = append(result, models.EnvVariable{
			Name:  name,
			Value: string(value),
		})
	}

	return result, nil
}

// EnvironmentSet adds or modifies the specified environment variable
// for the named application. When the function returns the variable
// will have the specified value. If the application is active the
// workload is restarted to update it to the new settings. The
// function will __not__ wait on this to complete.
func EnvironmentSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, assignments models.EnvVariableList) error {
	return envUpdate(ctx, cluster, appRef, func(evSecret *v1.Secret) {
		for _, ev := range assignments {
			evSecret.Data[ev.Name] = []byte(ev.Value)
		}
	})
}

// EnvironmentUnset removes the specified environment variable from the
// named application. When the function returns the variable will be
// gone. If the application is active the workload is restarted to
// update it to the new settings. The function will __not__ wait on
// this to complete.
func EnvironmentUnset(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, varName string) error {
	return envUpdate(ctx, cluster, appRef, func(evSecret *v1.Secret) {
		delete(evSecret.Data, varName)
	})
}

// envNames is a helper for envUpdate, it returns the names of all
// environment variable held by the given kube secret.
func envNames(ev *v1.Secret) []string {
	names := make([]string, len(ev.Data))
	i := 0
	for k := range ev.Data {
		names[i] = k
		i++
	}
	return names
}

// envUpdate is the helper for the public function encapsulating the
// read/modify/write cycle necessary to update the application's kube
// resource holding the application's environment, and the logic to
// restart the workload so that it may gain the changed settings.
func envUpdate(ctx context.Context, cluster *kubernetes.Cluster,
	appRef models.AppRef, modifyEnvironment func(*v1.Secret)) error {

	varNames := []string{}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		evSecret, err := envLoad(ctx, cluster, appRef)
		if err != nil {
			return err
		}

		if evSecret.Data == nil {
			evSecret.Data = make(map[string][]byte)
		}

		modifyEnvironment(evSecret)

		_, err = cluster.Kubectl.CoreV1().Secrets(appRef.Org).Update(
			ctx, evSecret, metav1.UpdateOptions{})

		// Pass current set of environment variables out for
		// use by the worload restart
		varNames = envNames(evSecret)

		return err
	})

	if err != nil {
		return err
	}

	// Restart the app workload, if it exists We ignore a missing deployment
	// as this just means that the EV changes will simply stand ready for
	// when the workload is actually launched.

	app, err := Lookup(ctx, cluster, appRef.Org, appRef.Name)
	if err != nil {
		return err
	}
	if app != nil {
		workload := NewWorkload(cluster, appRef)
		err = workload.EnvironmentChange(ctx, varNames)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
	}

	return nil
}

// envLoad locates and returns the kube secret storing the referenced
// application's environment. If necessary it creates that secret.
func envLoad(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*v1.Secret, error) {
	secretName := appRef.EnvSecret()

	evSecret, err := cluster.GetSecret(ctx, appRef.Org, secretName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}

		// Error is `Not Found`. Create the secret.

		app, err := Get(ctx, cluster, appRef)
		if err != nil {
			// Should not happen. Application was validated to exist
			// already somewhere by callers.
			return nil, err
		}

		owner := metav1.OwnerReference{
			APIVersion: app.GetAPIVersion(),
			Kind:       app.GetKind(),
			Name:       app.GetName(),
			UID:        app.GetUID(),
		}

		evSecret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: appRef.Org,
				OwnerReferences: []metav1.OwnerReference{
					owner,
				},
			},
		}
		err = cluster.CreateSecret(ctx, appRef.Org, *evSecret)

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

	return evSecret, nil
}
