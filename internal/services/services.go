package services

import (
	"context"
	"fmt"

	epinioappv1 "github.com/epinio/application/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	result, err := s.serviceKubeClient.Namespace("epinio").Get(ctx, serviceName, metav1.GetOptions{})
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
	listResult, err := s.serviceKubeClient.List(ctx, metav1.ListOptions{})
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

func (s *ServiceClient) CreateRelease(ctx context.Context, namespace, serviceName, releaseName string) error {
	serviceReleaseCR := &epinioappv1.ServiceRelease{
		TypeMeta: metav1.TypeMeta{
			APIVersion: epinioappv1.GroupVersion.String(),
			Kind:       "ServiceRelease",
		},
		ObjectMeta: metav1.ObjectMeta{Name: releaseName},
		Spec: epinioappv1.ServiceReleaseSpec{
			Name: serviceName,
		},
	}

	mapServiceRelease, err := runtime.DefaultUnstructuredConverter.ToUnstructured(serviceReleaseCR)
	if err != nil {
		return errors.Wrap(err, "error converting serviceReleaseCR to unstructured")
	}

	unstructureServiceRelease := &unstructured.Unstructured{}
	unstructureServiceRelease.SetUnstructuredContent(mapServiceRelease)

	_, err = s.serviceReleaseKubeClient.Namespace(namespace).Create(ctx, unstructureServiceRelease, metav1.CreateOptions{})
	return errors.Wrap(err, "error creating serviceReleaseCR")
}
