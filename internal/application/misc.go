package application

import (
	"context"
	"encoding/json"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// Note: This sub-module piggy-backs on the secret used to store scaling information,
// i.e. the number of desired instances. That secret contained only one key, for the
// instances. With this module it now contains to additional keys.

const (
	liveKey  = "health-live"
	readyKey = "health-ready"
)

// Liveness returns the application's liveness probe, if any.
func Liveness(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*models.ApplicationProbe, error) {
	return Probe(ctx, cluster, appRef, liveKey)
}

// Readiness returns the application's readiness probe, if any.
func Readiness(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*models.ApplicationProbe, error) {
	return Probe(ctx, cluster, appRef, readyKey)
}

// LivenessSet sets or removes the application's liveness probe
func LivenessSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, probe *models.ApplicationProbe) error {
	return ProbeSet(ctx, cluster, appRef, liveKey, probe)
}

// ReadinessSet sets or removes the application's readiness probe
func ReadinessSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, probe *models.ApplicationProbe) error {
	return ProbeSet(ctx, cluster, appRef, readyKey, probe)
}

// Probe returns the specified probe for the application, if specified.
func Probe(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, probe string) (*models.ApplicationProbe, error) {
	scaleSecret, err := scaleLoad(ctx, cluster, appRef)
	if err != nil {
		return nil, err
	}

	bytes, ok := scaleSecret.Data[probe]
	if !ok {
		return nil, nil
	}

	var probeSpec models.ApplicationProbe
	err = json.Unmarshal(bytes, &probeSpec)
	if err != nil {
		return nil, err
	}

	return &probeSpec, nil
}

// ProbeSet sets the specified probe for the named application.
// When the function returns the data is saved.
func ProbeSet(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, probe string, spec *models.ApplicationProbe) error {
	return probeUpdate(ctx, cluster, appRef, func(scaleSecret *v1.Secret) error {
		if spec == nil {
			delete(scaleSecret.Data, probe)
			return nil
		}

		bytes, err := json.Marshal(spec)
		if err != nil {
			return err
		}

		scaleSecret.Data[probe] = bytes
		return nil
	})
}

// probeUpdate is a helper for the public functions. It encapsulates the read/modify/write
// cycle necessary to update the application's kube resource holding the application's
// number of desired instances
func probeUpdate(ctx context.Context, cluster *kubernetes.Cluster,
	appRef models.AppRef, modifyScaling func(*v1.Secret) error) error {

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		scaleSecret, err := scaleLoad(ctx, cluster, appRef)
		if err != nil {
			return err
		}

		if scaleSecret.Data == nil {
			scaleSecret.Data = map[string][]byte{}
		}

		err = modifyScaling(scaleSecret)
		if err != nil {
			return err
		}

		_, err = cluster.Kubectl.CoreV1().Secrets(appRef.Org).Update(
			ctx, scaleSecret, metav1.UpdateOptions{})

		return err
	})
}
