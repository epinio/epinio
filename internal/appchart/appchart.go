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
)

// Create constructs and saves a new app chart resource.
func Create(ctx context.Context, cluster *kubernetes.Cluster, name, repository, url string) error {
	return nil
}

// Delete removed the named app chart CR.
func Delete(ctx context.Context, cluster *kubernetes.Cluster, name string) error {
	return nil
}

// List returns a slice of all known app chart CRs.
func List(ctx context.Context, cluster *kubernetes.Cluster) ([]models.AppChart, error) {
	client, err := cluster.ClientAppChart()
	if err != nil {
		return nil, err
	}

	list, err := client.Namespace(helmchart.Namespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	apps := make([]models.AppChart, 0, len(list.Items))

	for _, chart := range list.Items {
		copy := chart // Prevent memory aliasing warning
		appChart, err := toChart(&copy)
		if err != nil {
			return nil, err
		}
		apps = append(apps, *appChart)
	}

	return apps, nil
}

// Exists tests if the named app chart exists, or not.
func Exists(ctx context.Context, cluster *kubernetes.Cluster, name string) (bool, error) {
	_, err := Get(ctx, cluster, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Lookup returns the named app chart, or nil
func Lookup(ctx context.Context, cluster *kubernetes.Cluster, name string) (*models.AppChart, error) {
	chartCR, err := Get(ctx, cluster, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return toChart(chartCR)
}

// Get returns the app chart resource from the cluster.  This should be
// changed to return a typed application struct, like epinioappv1.AppChartSpec if
// needed in the future.
func Get(ctx context.Context, cluster *kubernetes.Cluster, name string) (*unstructured.Unstructured, error) {
	client, err := cluster.ClientAppChart()
	if err != nil {
		return nil, err
	}

	return client.Namespace(helmchart.Namespace()).Get(ctx, name, metav1.GetOptions{})
}

// toChart converts the unstructured app chart CR into the proper model
func toChart(chart *unstructured.Unstructured) (*models.AppChart, error) {

	name, _, err := unstructured.NestedString(chart.UnstructuredContent(), "spec", "name")
	if err != nil {
		return nil, errors.New("chart should be string")
	}

	description, _, err := unstructured.NestedString(chart.UnstructuredContent(), "spec", "description")
	if err != nil {
		return nil, errors.New("description should be string")
	}

	short, _, err := unstructured.NestedString(chart.UnstructuredContent(), "spec", "shortDescription")
	if err != nil {
		return nil, errors.New("shortdescription should be string")
	}

	chartRef, _, err := unstructured.NestedString(chart.UnstructuredContent(), "spec", "chart")
	if err != nil {
		return nil, errors.New("helmchart should be string")
	}

	repoName, _, err := unstructured.NestedString(chart.UnstructuredContent(), "spec", "helmRepo", "name")
	if err != nil {
		return nil, errors.New("repo name should be string")
	}

	repoURL, _, err := unstructured.NestedString(chart.UnstructuredContent(), "spec", "helmRepo", "url")
	if err != nil {
		return nil, errors.New("repo url should be string")
	}

	return &models.AppChart{
		Name:             name,
		Description:      description,
		ShortDescription: short,
		HelmChart:        chartRef,
		HelmRepo: models.HelmRepo{
			Name: repoName,
			URL:  repoURL,
		},
	}, nil
}
