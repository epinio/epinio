package application

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/routes"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

// DesiredRoutes lists all desired routes for the given application
// The list is constructed from the stored information on the
// Application Custom Resource.
func DesiredRoutes(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) ([]string, error) {
	applicationCR, err := Get(ctx, cluster, appRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []string{}, apierror.AppIsNotKnown("application resource is missing")
		}
		return []string{}, apierror.InternalError(err, "failed to get the application resource")
	}

	desiredRoutes, found, err := unstructured.NestedStringSlice(applicationCR.Object, "spec", "routes")
	if !found {
		// [NO-ROUTES] Not an error. Signal that there are no desired routes.  See `Create`
		// for the converse. An empty slice becomes an omitted field. Same marker as here.
		return []string{}, nil
	}
	if err != nil {
		return []string{}, err
	}

	return desiredRoutes, nil
}

// ListRoutes lists all (currently active) routes for the given application
// The list is constructed from the actual Ingresses and not from the stored
// information on the Application Custom Resource.
func ListRoutes(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) ([]string, error) {
	ingressList, err := ingressListForApp(ctx, cluster, appRef)
	if err != nil {
		return []string{}, err
	}

	result := []string{}
	for _, ingress := range ingressList.Items {
		routes, err := routes.FromIngress(ingress)
		if err != nil {
			return result, err
		}
		for _, r := range routes {
			result = append(result, r.String())
		}
	}

	return result, nil
}

func ingressListForApp(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (*networkingv1.IngressList, error) {
	// Find all user credential secrets
	ingressSelector := labels.Set(map[string]string{
		"app.kubernetes.io/name": appRef.Name,
	}).AsSelector().String()

	return cluster.Kubectl.NetworkingV1().Ingresses(appRef.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: ingressSelector,
	})
}
