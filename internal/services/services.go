package services

import (
	"context"
	"fmt"

	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewServiceFromJSONMap(m map[string]interface{}) (*models.Service, error) {
	var err error
	service := &models.Service{}

	if service.Name, _, err = unstructured.NestedString(m, "spec", "name"); err != nil {
		return nil, errors.New("name should be string")
	}

	if service.ShortDescription, _, err = unstructured.NestedString(m, "spec", "shortDescription"); err != nil {
		return nil, errors.New("shortDescription should be string")
	}

	if service.Description, _, err = unstructured.NestedString(m, "spec", "description"); err != nil {
		return nil, errors.New("description should be string")
	}

	if service.HelmChart, _, err = unstructured.NestedString(m, "spec", "chart"); err != nil {
		return nil, errors.New("chart should be string")
	}

	if service.HelmRepo.Name, _, err = unstructured.NestedString(m, "spec", "helmRepo", "name"); err != nil {
		return nil, errors.New("helmRepo.name should be string")
	}

	if service.HelmRepo.URL, _, err = unstructured.NestedString(m, "spec", "helmRepo", "url"); err != nil {
		return nil, errors.New("helmRepo.url should be string")
	}

	if service.Values, _, err = unstructured.NestedString(m, "spec", "values"); err != nil {
		return nil, errors.New("values should be string")
	}

	return service, nil
}

func (s *ServiceClient) Get(ctx context.Context, serviceName string) (*models.Service, error) {
	result, err := s.serviceKubeClient.Namespace(helmchart.Namespace()).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error getting service %s from namespace epinio", serviceName))
	}

	service, err := NewServiceFromJSONMap(result.UnstructuredContent())
	if err != nil {
		return nil, errors.Wrap(err, "error creating Service from JSON map")
	}
	return service, nil
}

func (s *ServiceClient) List(ctx context.Context) ([]*models.Service, error) {
	listResult, err := s.serviceKubeClient.Namespace(helmchart.Namespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error listing services")
	}

	services := []*models.Service{}
	for _, item := range listResult.Items {
		service, err := NewServiceFromJSONMap(item.UnstructuredContent())
		if err != nil {
			return nil, errors.Wrap(err, "error creating Service from JSON map")
		}
		services = append(services, service)
	}

	return services, nil
}
