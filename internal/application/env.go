package application

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func EnvironmentSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, assignments models.EnvVariableList) error {
	// get env from secret
	// merge assignments
	// update secret
	// restart workload, if any
	return nil
}

func EnvironmentUnset(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, varName string) error {
	// get env from secret
	// remove assignment
	// update secret
	// restart workload, if any
	return nil
}

func envLoad(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*v1.Secret, error) {
	secretName := appRef.Name + "-env"

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
