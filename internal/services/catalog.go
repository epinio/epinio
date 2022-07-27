package services

import (
	"context"
	"fmt"
	"strings"

	apiv1 "github.com/epinio/application/api/v1"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// Only Helmcharts with this label are considered Epinio "Services".
	// Used to filter out Helmcharts created by other means (manually, k3s etc).
	CatalogServiceLabelKey              = "application.epinio.io/catalog-service-name"
	CatalogServiceSecretTypesAnnotation = "application.epinio.io/catalog-service-secret-types"
	CatalogServiceVersionLabelKey       = "application.epinio.io/catalog-service-version"
	TargetNamespaceLabelKey             = "application.epinio.io/target-namespace"
	// ServiceNameLabelKey is used to keep the original name
	// since the name in the metadata is combined with the namespace
	ServiceNameLabelKey = "application.epinio.io/service-name"
)

func (s *ServiceClient) GetCatalogService(ctx context.Context, serviceName string) (*models.CatalogService, error) {
	result, err := s.serviceKubeClient.Namespace(helmchart.Namespace()).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error getting service %s from namespace epinio", serviceName))
	}

	service, err := convertUnstructuredIntoCatalogService(*result)
	if err != nil {
		return nil, errors.Wrap(err, "error converting result into Catalog Service")
	}

	return service, nil
}

func (s *ServiceClient) ListCatalogServices(ctx context.Context) ([]*models.CatalogService, error) {
	listResult, err := s.serviceKubeClient.Namespace(helmchart.Namespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error listing services")
	}

	services, err := convertUnstructuredListIntoCatalogService(listResult)
	if err != nil {
		return nil, errors.Wrap(err, "error converting listResult into Catalog Services")
	}

	return services, nil
}

func convertUnstructuredListIntoCatalogService(unstructuredList *unstructured.UnstructuredList) ([]*models.CatalogService, error) {
	catalogServices := []*models.CatalogService{}

	for _, item := range unstructuredList.Items {
		catalogService, err := convertUnstructuredIntoCatalogService(item)
		if err != nil {
			return nil, errors.Wrap(err, "error converting catalog service list")
		}
		catalogServices = append(catalogServices, catalogService)
	}

	return catalogServices, nil
}

func convertUnstructuredIntoCatalogService(unstructured unstructured.Unstructured) (*models.CatalogService, error) {
	catalogService := apiv1.Service{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, &catalogService)
	if err != nil {
		return nil, errors.Wrap(err, "error converting catalog service")
	}

	serviceSecretTypes := catalogService.GetAnnotations()[CatalogServiceSecretTypesAnnotation]

	return &models.CatalogService{
		Meta: models.MetaLite{
			Name:      catalogService.Spec.Name,
			CreatedAt: unstructured.GetCreationTimestamp(),
		},
		SecretTypes:      strings.Split(serviceSecretTypes, ","),
		Description:      catalogService.Spec.Description,
		ShortDescription: catalogService.Spec.ShortDescription,
		HelmChart:        catalogService.Spec.HelmChart,
		ChartVersion:     catalogService.Spec.ChartVersion,
		ServiceIcon:      catalogService.Spec.ServiceIcon,
		AppVersion:       catalogService.Spec.AppVersion,
		HelmRepo: models.HelmRepo{
			Name: catalogService.Spec.HelmRepo.Name,
			URL:  catalogService.Spec.HelmRepo.URL,
		},
		Values: catalogService.Spec.Values,
	}, nil
}
