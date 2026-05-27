// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package appchart collects the structures and functions that deal with epinio's app chart CR
package appchart

import (
	"context"
	"errors"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// LookupViaCluster is a convenience wrapper for callers that have a
// *kubernetes.Cluster but not a dynamic client. It resolves the dynamic
// client from the cluster and delegates to Lookup.
func LookupViaCluster(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	name string,
) (*models.AppChartFull, error) {
	client, clientError := cluster.ClientAppChart()
	if clientError != nil {
		return nil, clientError
	}
	return Lookup(ctx, client, name)
}

// ExistsViaCluster is a convenience wrapper that mirrors LookupViaCluster
// for the Exists check.
func ExistsViaCluster(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	name string,
) (bool, error) {
	client, clientError := cluster.ClientAppChart()
	if clientError != nil {
		return false, clientError
	}
	return Exists(ctx, client, name)
}

// List returns a slice of all known app chart CRs.
func List(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
) (models.AppChartList, error) {
	list, listError := client.Namespace(helmchart.Namespace()).List(
		ctx,
		metav1.ListOptions{},
	)
	if listError != nil {
		return nil, listError
	}

	apps := make(models.AppChartList, 0, len(list.Items))

	for _, chart := range list.Items {
		copyChart := chart // Prevent memory aliasing warning
		converted, convertError := toChart(&copyChart)
		if convertError != nil {
			return nil, convertError
		}
		apps = append(apps, *converted)
	}

	return apps, nil
}

// Exists tests if the named app chart exists, or not.
func Exists(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	name string,
) (bool, error) {
	_, getError := Get(ctx, client, name)
	if getError != nil {
		if apierrors.IsNotFound(getError) {
			return false, nil
		}
		return false, getError
	}
	return true, nil
}

// Lookup returns the named app chart, or nil
func Lookup(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	name string,
) (*models.AppChartFull, error) {
	chartCR, getError := Get(ctx, client, name)
	if getError != nil {
		if apierrors.IsNotFound(getError) {
			return nil, nil
		}
		return nil, getError
	}

	return toChart(chartCR)
}

// Get returns the app chart resource from the cluster.
func Get(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	name string,
) (*unstructured.Unstructured, error) {
	return client.Namespace(helmchart.Namespace()).Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
}

// toChart converts the unstructured app chart CR into the proper model
func toChart(chart *unstructured.Unstructured) (*models.AppChartFull, error) {

	name, _, nameError := unstructured.NestedString(
		chart.UnstructuredContent(),
		"metadata",
		"name",
	)
	if nameError != nil {
		return nil, errors.New("chart should be string")
	}

	description, _, descriptionError := unstructured.NestedString(
		chart.UnstructuredContent(),
		"spec",
		"description",
	)
	if descriptionError != nil {
		return nil, errors.New("description should be string")
	}

	short, _, shortError := unstructured.NestedString(
		chart.UnstructuredContent(),
		"spec",
		"shortDescription",
	)
	if shortError != nil {
		return nil, errors.New("shortdescription should be string")
	}

	helmChart, _, helmChartError := unstructured.NestedString(
		chart.UnstructuredContent(),
		"spec",
		"helmChart",
	)
	if helmChartError != nil {
		return nil, errors.New("helm chart should be string")
	}

	helmRepo, _, helmRepoError := unstructured.NestedString(
		chart.UnstructuredContent(),
		"spec",
		"helmRepo",
	)
	if helmRepoError != nil {
		return nil, errors.New("helm repo should be string")
	}

	theValues, _, valuesError := unstructured.NestedStringMap(
		chart.UnstructuredContent(),
		"spec",
		"values",
	)
	if valuesError != nil {
		return nil, errors.New("spec values should be map")
	}

	settings, settingsError := helmchart.SettingsToChart(chart)
	if settingsError != nil {
		return nil, settingsError
	}

	createdAt := chart.GetCreationTimestamp()

	return &models.AppChartFull{
		AppChart: models.AppChart{
			Meta: models.MetaLite{
				Name:      name,
				CreatedAt: createdAt,
			},
			Description:      description,
			ShortDescription: short,
			HelmChart:        helmChart,
			HelmRepo:         helmRepo,
			Settings:         settings,
		},
		Values: theValues,
	}, nil
}
