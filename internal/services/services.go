package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewServiceFromJSONMap(m map[string]interface{}) (*models.Service, error) {
	var err error
	service := &models.Service{}

	if service.Name, _, err = unstructured.NestedString(m, "spec", "name"); err != nil {
		return nil, errors.New("name should be string")
	}

	if service.Description, _, err = unstructured.NestedString(m, "spec", "description"); err != nil {
		return nil, errors.New("description should be string")
	}

	if service.HelmChart, _, err = unstructured.NestedString(m, "spec", "chart"); err != nil {
		return nil, errors.New("chart should be string")
	}

	if service.HelmRepo.Name, _, err = unstructured.NestedString(m, "spec", "helmRepo", "name"); err != nil {
		return nil, errors.New("chart should be string")
	}

	if service.HelmRepo.URL, _, err = unstructured.NestedString(m, "spec", "helmRepo", "url"); err != nil {
		return nil, errors.New("chart should be string")
	}

	if service.Values, _, err = unstructured.NestedString(m, "spec", "values"); err != nil {
		return nil, errors.New("values should be string")
	}

	if service.UserValues, _, err = unstructured.NestedString(m, "spec", "userValues"); err != nil {
		return nil, errors.New("userValues should be string")
	}

	return service, nil
}

func (s *ServiceClient) Get(ctx context.Context, serviceName string) (*models.Service, error) {
	serviceList, err := s.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, service := range serviceList {
		if service.Name == serviceName {
			return service, nil
		}
	}

	return nil, fmt.Errorf("service %s not found", serviceName)
}

// TODO fix
// func (s *ServiceClient) Get(ctx context.Context, serviceName string) (*models.Service, error) {
// 	result, err := s.serviceKubeClient.Get(ctx, serviceName, metav1.GetOptions{})
// 	if err != nil {
// 		return nil, err
// 	}

// 	service, err := NewServiceFromJSONMap(result.UnstructuredContent())
// 	if err != nil {
// 		return nil, err
// 	}
// 	return service, nil
// }

func (s *ServiceClient) List(ctx context.Context) ([]*models.Service, error) {
	listResult, err := s.serviceKubeClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	services := []*models.Service{}
	for _, item := range listResult.Items {
		service, err := NewServiceFromJSONMap(item.UnstructuredContent())
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	return services, nil
}
