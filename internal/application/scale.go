package application

import (
	"context"
	"math"
	"strconv"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

const (
	instanceKey = "desired"
)

// Scaling returns the number of desired instances set by a user for the application
func Scaling(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (int32, error) {
	scaleSecret, err := scaleLoad(ctx, cluster, appRef)
	if err != nil {
		return 0, err
	}

	result, err := strconv.Atoi(string(scaleSecret.Data[instanceKey])) // nolint:gosec // overflow blocked by guards
	if err != nil {
		return 0, err
	}

	// Reject bad values, and assume single instance - Return err better ? Save back, fix resource ?
	if result <= 0 || result > math.MaxInt32 {
		result = 1
	}

	return int32(result), nil
}

// ScalingSet sets the desired number of instances for the named application.
// When the function returns the number is saved.
func ScalingSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, instances int32) error {
	return scaleUpdate(ctx, cluster, appRef, func(scaleSecret *v1.Secret) {
		scaleSecret.Data[instanceKey] = []byte(strconv.Itoa(int(instances)))
	})
}

// scaleUpdate is a helper for the public functions. It encapsulates the read/modify/write cycle
// necessary to update the application's kube resource holding the application's number of desired
// instances
func scaleUpdate(ctx context.Context, cluster *kubernetes.Cluster,
	appRef models.AppRef, modifyScaling func(*v1.Secret)) error {

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		scaleSecret, err := scaleLoad(ctx, cluster, appRef)
		if err != nil {
			return err
		}

		if scaleSecret.Data == nil {
			scaleSecret.Data = map[string][]byte{
				instanceKey: []byte(`1`),
			}
		}

		modifyScaling(scaleSecret)

		_, err = cluster.Kubectl.CoreV1().Secrets(appRef.Org).Update(
			ctx, scaleSecret, metav1.UpdateOptions{})

		return err
	})
}

// scaleLoad locates and returns the kube secret storing the referenced application's desired number of
// instances. If necessary it creates that secret.
func scaleLoad(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*v1.Secret, error) {
	secretName := appRef.MakeScaleSecretName()

	scaleSecret, err := cluster.GetSecret(ctx, appRef.Org, secretName)
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

		scaleSecret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: appRef.Org,
				OwnerReferences: []metav1.OwnerReference{
					owner,
				},
			},
		}
		err = cluster.CreateSecret(ctx, appRef.Org, *scaleSecret)

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

	return scaleSecret, nil
}
