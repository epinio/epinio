package appchart

import (
	"context"

	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

const KIND = "AppChart"
const API_VERSION = "application.epinio.io/v1"
const NAMESPACE = "epinio"

func Create(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	chart models.AppChartCreateRequest,
) (*unstructured.Unstructured, error) {

	finalChart := &unstructured.Unstructured{Object: map[string]interface{}{}}

	finalChart.SetKind(KIND)
	finalChart.SetAPIVersion(API_VERSION)
	finalChart.SetName(chart.Name)
	finalChart.SetNamespace(NAMESPACE)

	// Set the labels so Epinio and kubectl can filter/find it
	finalChart.SetLabels(map[string]string{
		"app.kubernetes.io/managed-by": NAMESPACE,
		"epinio.io/area":               NAMESPACE,
	})

	// Build the spec with the CRD's camelCase keys explicitly. The request DTO
	// carries snake_case JSON tags for the client API, so it cannot be fed to
	// the unstructured converter directly, that would emit snake_case spec
	// keys the AppChart CRD rejects. Name is the metadata name only, never a
	// spec field. Mirrors the explicit mapping in appchart_update.go.
	spec := map[string]interface{}{}
	if chart.Description != "" {
		spec["description"] = chart.Description
	}
	if chart.ShortDescription != "" {
		spec["shortDescription"] = chart.ShortDescription
	}
	if chart.HelmChart != "" {
		spec["helmChart"] = chart.HelmChart
	}
	if chart.HelmRepo != "" {
		spec["helmRepo"] = chart.HelmRepo
	}
	finalChart.Object["spec"] = spec

	if chart.Values != nil {
		valuesError := unstructured.SetNestedStringMap(
			finalChart.Object,
			chart.Values,
			"spec",
			"values",
		)
		if valuesError != nil {
			return nil, valuesError
		}
	}
	if chart.Settings != nil {
		if patchError := patchSettings(finalChart, chart.Settings); patchError != nil {
			return nil, patchError
		}
	}

	appChartFull, createError := client.
		Namespace(helmchart.Namespace()).
		Create(ctx, finalChart, metav1.CreateOptions{})
	if createError != nil {
		return nil, createError
	}

	return appChartFull, nil
}
