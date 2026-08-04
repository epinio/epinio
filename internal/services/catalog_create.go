package services

import (
	"context"
	"strings"

	apiv1 "github.com/epinio/application/api/v1"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// CreateCatalogService lands a new Service CR (catalog entry) in the
// epinio namespace.
func (s *ServiceClient) CreateCatalogService(
	ctx context.Context,
	req models.CatalogServiceCreateRequest,
) (*unstructured.Unstructured, error) {
	cr := apiv1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "application.epinio.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: helmchart.Namespace(),
		},
		Spec: apiv1.ServiceSpec{
			Name:             req.Name,
			ShortDescription: req.ShortDescription,
			Description:      req.Description,
			HelmChart:        req.HelmChart,
			ChartVersion:     req.ChartVersion,
			AppVersion:       req.AppVersion,
			ServiceIcon:      req.ServiceIcon,
			Values:           req.Values,
			HelmRepo: apiv1.HelmRepo{
				Name:   req.HelmRepo.Name,
				URL:    req.HelmRepo.URL,
				Secret: req.HelmRepo.Secret,
			},
			Settings: convertChartSettingsToCRD(req.Settings),
		},
	}

	content, contentError := runtime.
		DefaultUnstructuredConverter.
		ToUnstructured(&cr)
	if contentError != nil {
		return nil, errors.Wrap(contentError, "converting catalog service to unstructured")
	}

	unstructuredCR := &unstructured.Unstructured{Object: content}

	annotations := map[string]string{}
	if len(req.SecretTypes) > 0 {
		annotations[CatalogServiceSecretTypesAnnotation] =
			strings.Join(req.SecretTypes, ",")
	}
	if len(annotations) > 0 {
		unstructuredCR.SetAnnotations(annotations)
	}

	created, createError := s.serviceKubeClient.
		Namespace(helmchart.Namespace()).
		Create(ctx, unstructuredCR, metav1.CreateOptions{})
	if createError != nil {
		return nil, errors.Wrap(createError, "creating catalog service")
	}

	return created, nil
}
