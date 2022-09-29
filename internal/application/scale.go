package application

import (
	"context"
	"math"
	"strconv"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	v1 "k8s.io/api/core/v1"
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

	result, err := strconv.Atoi(string(scaleSecret.Data[instanceKey]))
	if err != nil {
		return 0, err
	}

	// Reject bad values, and assume single instance - Return err better ? Save back, fix resource ?
	if result < 0 || result > math.MaxInt32 {
		result = 1
	}

	return int32(result), nil // nolint:gosec // overflow blocked by guards
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

		_, err = cluster.Kubectl.CoreV1().Secrets(appRef.Namespace).Update(
			ctx, scaleSecret, metav1.UpdateOptions{})

		return err
	})
}

// scaleLoad locates and returns the kube secret storing the referenced application's desired number of
// instances. If necessary it creates that secret.
func scaleLoad(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*v1.Secret, error) {
	secretName := appRef.MakeScaleSecretName()
	return loadOrCreateSecret(ctx, cluster, appRef, secretName, "scaling")
}
