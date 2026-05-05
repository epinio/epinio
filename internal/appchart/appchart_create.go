package appchart

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Create(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	chart *unstructured.Unstructured,
) (*models.AppChartFull, error) {

	client, clientError := cluster.ClientAppChart()
	if clientError != nil {
		return nil, clientError
	}

	_, createError := client.Namespace(helmchart.Namespace()).Create(
		ctx,
		chart,
		metav1.CreateOptions{},
	)

	if createError != nil {
		return nil, createError
	}

	return nil, nil
}
