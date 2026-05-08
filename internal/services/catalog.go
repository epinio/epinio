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

package services

import (
	"context"
	"fmt"
	"strings"

	apiv1 "github.com/epinio/application/api/v1"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	k8sapierrors "k8s.io/apimachinery/pkg/api/errors"
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
	// COMPATIBILITY SUPPORT for services from before https://github.com/epinio/epinio/issues/1704 fix
	TargetNamespaceLabelKey = "application.epinio.io/target-namespace"
	// ServiceNameLabelKey is used to keep the original name
	// since the name in the metadata is combined with the namespace
	ServiceNameLabelKey = "application.epinio.io/service-name"
)

// CatalogServiceExists tests if the named catalog service exists, or not.
func (s *ServiceClient) CatalogServiceExists(
	ctx context.Context,
	name string,
) (bool, error) {
	_, getError := s.serviceKubeClient.
		Namespace(helmchart.Namespace()).
		Get(ctx, name, metav1.GetOptions{})
	if getError != nil {
		if k8sapierrors.IsNotFound(getError) {
			return false, nil
		}
		return false, getError
	}
	return true, nil
}

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

// UpdateCatalogService applies a partial update to an existing catalog
// service CR. Empty primitive fields are left untouched. Settings and
// SecretTypes are replaced when non-nil.
func (s *ServiceClient) UpdateCatalogService(
	ctx context.Context,
	name string,
	req models.CatalogServiceUpdateRequest,
) error {
	existing, getError := s.serviceKubeClient.
		Namespace(helmchart.Namespace()).
		Get(ctx, name, metav1.GetOptions{})
	if getError != nil {
		return errors.Wrap(getError, "fetching catalog service for update")
	}

	spec, ok := existing.Object["spec"].(map[string]interface{})
	if !ok {
		spec = map[string]interface{}{}
		existing.Object["spec"] = spec
	}

	if req.ShortDescription != "" {
		spec["shortDescription"] = req.ShortDescription
	}
	if req.Description != "" {
		spec["description"] = req.Description
	}
	if req.HelmChart != "" {
		spec["chart"] = req.HelmChart
	}
	if req.ChartVersion != "" {
		spec["chartVersion"] = req.ChartVersion
	}
	if req.AppVersion != "" {
		spec["appVersion"] = req.AppVersion
	}
	if req.ServiceIcon != "" {
		spec["serviceIcon"] = req.ServiceIcon
	}
	if req.Values != "" {
		spec["values"] = req.Values
	}
	if req.HelmRepo != nil {
		spec["helmRepo"] = map[string]interface{}{
			"name":   req.HelmRepo.Name,
			"url":    req.HelmRepo.URL,
			"secret": req.HelmRepo.Secret,
		}
	}
	if req.Settings != nil {
		settingsMap := map[string]interface{}{}
		for key, value := range req.Settings {
			settingsMap[key] = map[string]interface{}{
				"type":    value.Type,
				"minimum": value.Minimum,
				"maximum": value.Maximum,
				"enum":    value.Enum,
			}
		}
		spec["settings"] = settingsMap
	}

	if req.SecretTypes != nil {
		annotations := existing.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		if len(req.SecretTypes) == 0 {
			delete(annotations, CatalogServiceSecretTypesAnnotation)
		} else {
			annotations[CatalogServiceSecretTypesAnnotation] =
				strings.Join(req.SecretTypes, ",")
		}
		existing.SetAnnotations(annotations)
	}

	_, updateError := s.serviceKubeClient.
		Namespace(helmchart.Namespace()).
		Update(ctx, existing, metav1.UpdateOptions{})
	if updateError != nil {
		return errors.Wrap(updateError, "updating catalog service")
	}

	return nil
}

// DeleteCatalogService removes the named Service CR from the epinio
// namespace.
func (s *ServiceClient) DeleteCatalogService(
	ctx context.Context,
	name string,
) error {
	deleteError := s.serviceKubeClient.
		Namespace(helmchart.Namespace()).
		Delete(ctx, name, metav1.DeleteOptions{})
	if deleteError != nil {
		return errors.Wrap(deleteError, "deleting catalog service")
	}
	return nil
}

// convertChartSettingsToCRD lifts the public ChartSetting map into the
// apiv1.ServiceSetting map shape expected by the CR.
func convertChartSettingsToCRD(
	settings map[string]models.ChartSetting,
) map[string]apiv1.ServiceSetting {
	if len(settings) == 0 {
		return nil
	}
	result := map[string]apiv1.ServiceSetting{}
	for key, value := range settings {
		result[key] = apiv1.ServiceSetting{
			Type:    value.Type,
			Minimum: value.Minimum,
			Maximum: value.Maximum,
			Enum:    value.Enum,
		}
	}
	return result
}

func (s *ServiceClient) GetCatalogService(ctx context.Context, serviceName string) (*models.CatalogService, error) {
	result, err := s.serviceKubeClient.Namespace(helmchart.Namespace()).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("error getting service %s from namespace epinio", serviceName))
	}

	service, err := s.convertUnstructuredIntoCatalogService(*result)
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

	services, err := s.convertUnstructuredListIntoCatalogService(listResult)
	if err != nil {
		return nil, errors.Wrap(err, "error converting listResult into Catalog Services")
	}

	return services, nil
}

func (s *ServiceClient) convertUnstructuredListIntoCatalogService(unstructuredList *unstructured.UnstructuredList) ([]*models.CatalogService, error) {
	catalogServices := []*models.CatalogService{}

	for _, item := range unstructuredList.Items {
		catalogService, err := s.convertUnstructuredIntoCatalogService(item)
		if err != nil {
			return nil, errors.Wrap(err, "error converting catalog service list")
		}
		catalogServices = append(catalogServices, catalogService)
	}

	return catalogServices, nil
}

func (s *ServiceClient) convertUnstructuredIntoCatalogService(unstructured unstructured.Unstructured) (*models.CatalogService, error) {
	catalogService := apiv1.Service{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.Object, &catalogService)
	if err != nil {
		return nil, errors.Wrap(err, "error converting catalog service")
	}

	// Convert from CRD structure to internal model
	settings := make(map[string]models.ChartSetting)
	for key, value := range catalogService.Spec.Settings {
		settings[key] = models.ChartSetting{
			Type:    value.Type,
			Minimum: value.Minimum,
			Maximum: value.Maximum,
			Enum:    value.Enum,
		}
	}

	// if a secret was specified try to load the credentials from it
	var repoUsername, repoPassword string
	if catalogService.Spec.HelmRepo.Secret != "" {
		authSecret, err := s.kubeClient.GetSecret(
			context.Background(),
			helmchart.Namespace(),
			catalogService.Spec.HelmRepo.Secret,
		)
		if err != nil {
			return nil, errors.Wrap(err, "finding helm repo auth secret: "+catalogService.Spec.HelmRepo.Secret)
		}

		repoUsername = string(authSecret.Data["username"])
		repoPassword = string(authSecret.Data["password"])
	}

	secretTypes := []string{}
	secretTypesAnnotationValue := catalogService.GetAnnotations()[CatalogServiceSecretTypesAnnotation]
	if len(secretTypesAnnotationValue) > 0 {
		secretTypes = strings.Split(secretTypesAnnotationValue, ",")
	}

	return &models.CatalogService{
		Meta: models.MetaLite{
			Name:      unstructured.GetName(),
			CreatedAt: unstructured.GetCreationTimestamp(),
		},
		SecretTypes:      secretTypes,
		Description:      catalogService.Spec.Description,
		ShortDescription: catalogService.Spec.ShortDescription,
		HelmChart:        catalogService.Spec.HelmChart,
		ChartVersion:     catalogService.Spec.ChartVersion,
		ServiceIcon:      catalogService.Spec.ServiceIcon,
		AppVersion:       catalogService.Spec.AppVersion,
		HelmRepo: models.HelmRepo{
			Name: catalogService.Spec.HelmRepo.Name,
			URL:  catalogService.Spec.HelmRepo.URL,
			Auth: models.HelmAuth{
				Username: repoUsername,
				Password: repoPassword,
			},
		},
		Values:   catalogService.Spec.Values,
		Settings: settings,
	}, nil
}
