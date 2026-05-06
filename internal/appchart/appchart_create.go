package appchart

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const KIND = "AppChart"
const API_VERSION = "application.epinio.io/v1"
const NAMESPACE = "epinio"

func Create(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	chart models.AppChartCreateRequest,
) (*unstructured.Unstructured, error) {

	//Parse the request into a unstructured object, so we can create it in the
	//cluster.
	var appChartRequest = &models.AppChartRequest{
		Spec: chart,
	}

	content, contentError := runtime.
		DefaultUnstructuredConverter.
		ToUnstructured(appChartRequest)

	if contentError != nil {
		return nil, contentError
	}

	finalChart := &unstructured.Unstructured{Object: content}

	finalChart.SetKind(KIND)
	finalChart.SetAPIVersion(API_VERSION)

	finalChart.SetName(chart.Name)
	finalChart.SetNamespace(NAMESPACE)

	// Set the labels so Epinio and kubectl can filter/find it
	finalChart.SetLabels(map[string]string{
		"app.kubernetes.io/managed-by": NAMESPACE,
		"epinio.io/area":               NAMESPACE,
	})
	client, clientError := cluster.ClientAppChart()
	if clientError != nil {
		return nil, clientError
	}

	//Create the AppChart in the cluster and return it.
	appChartFull, createError := client.Namespace(helmchart.Namespace()).Create(
		ctx,
		finalChart,
		metav1.CreateOptions{},
	)

	if createError != nil {
		return nil, createError
	}

	return appChartFull, nil
}
