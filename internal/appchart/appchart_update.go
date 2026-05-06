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

func Update(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	name string,
	req models.AppChartUpdateRequest,
) error {
	client, err := cluster.ClientAppChart()
	if err != nil {
		return err
	}

	existing, existsError := client.
		Namespace(helmchart.Namespace()).
		Get(ctx, name, metav1.GetOptions{})

	if existsError != nil {
		return err
	}

	spec := existing.Object["spec"].(map[string]interface{})

	if req.Description != "" {
		spec["description"] = req.Description
	}
	if req.ShortDescription != "" {
		spec["shortDescription"] = req.ShortDescription
	}
	if req.HelmChart != "" {
		spec["helmChart"] = req.HelmChart
	}
	if req.HelmRepo != "" {
		spec["helmRepo"] = req.HelmRepo
	}
	if req.Values != nil {
		if err := unstructured.SetNestedStringMap(existing.Object, req.Values, "spec", "values"); err != nil {
			return err
		}
	}
	if req.Settings != nil {
		patchError := patchSettings(existing, req.Settings)
		if patchError != nil {
			return patchError
		}
	}

	_, updateError := client.
		Namespace(helmchart.Namespace()).
		Update(ctx, existing, metav1.UpdateOptions{})

	return updateError
}

func patchSettings(
	chart *unstructured.Unstructured,
	settings map[string]models.ChartSetting,
) error {
	settingsMap := make(map[string]interface{}, len(settings))

	for key, value := range settings {
		converted, convertError := runtime.
			DefaultUnstructuredConverter.
			ToUnstructured(&value)

		if convertError != nil {
			return convertError
		}
		settingsMap[key] = converted
	}

	return unstructured.SetNestedMap(
		chart.Object,
		settingsMap,
		"spec",
		"settings",
	)
}
