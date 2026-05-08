package builderimage

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const KIND = "BuilderImage"
const API_VERSION = "application.epinio.io/v1"
const NAMESPACE = "epinio"

// Create lands a new BuilderImage CR in the cluster.
func Create(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	bp models.BuilderImageCreateRequest,
) (*unstructured.Unstructured, error) {

	request := &models.BuilderImageRequest{
		Spec: bp,
	}

	content, err := runtime.
		DefaultUnstructuredConverter.
		ToUnstructured(request)
	if err != nil {
		return nil, err
	}

	final := &unstructured.Unstructured{Object: content}

	final.SetKind(KIND)
	final.SetAPIVersion(API_VERSION)
	final.SetName(bp.Name)
	final.SetNamespace(NAMESPACE)

	final.SetLabels(map[string]string{
		"app.kubernetes.io/managed-by": NAMESPACE,
		"epinio.io/area":               NAMESPACE,
	})

	client, err := cluster.ClientBuilderImage()
	if err != nil {
		return nil, err
	}

	created, err := client.Namespace(helmchart.Namespace()).Create(
		ctx,
		final,
		metav1.CreateOptions{},
	)
	if err != nil {
		return nil, err
	}

	return created, nil
}
