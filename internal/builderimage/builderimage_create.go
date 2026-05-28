package builderimage

import (
	"context"

	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

const KIND = "BuilderImage"
const API_VERSION = "application.epinio.io/v1"
const NAMESPACE = "epinio"

// Create lands a new BuilderImage CR in the cluster.
func Create(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	bp models.BuilderImageCreateRequest,
) (*unstructured.Unstructured, error) {

	request := &models.BuilderImageRequest{
		Spec: bp,
	}

	content, contentError := runtime.
		DefaultUnstructuredConverter.
		ToUnstructured(request)
	if contentError != nil {
		return nil, contentError
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

	created, createError := client.
		Namespace(helmchart.Namespace()).
		Create(ctx, final, metav1.CreateOptions{})
	if createError != nil {
		return nil, createError
	}

	return created, nil
}
