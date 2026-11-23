// Copyright © 2021 - 2026 KRUMWARE LLC
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
	"github.com/epinio/epinio/helpers"
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
	// COMPATIBILITY SUPPORT for services from before
	// https://github.com/epinio/epinio/issues/1704 fix
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

// chartSettingsToUnstructuredMap converts API chart settings into an
// unstructured map, including optional description/example fields.
func chartSettingsToUnstructuredMap(
	settings map[string]models.ChartSetting,
) (map[string]interface{}, error) {
	settingsMap := make(map[string]interface{}, len(settings))
	for key, value := range settings {
		converted, convertError := runtime.
			DefaultUnstructuredConverter.
			ToUnstructured(&value)
		if convertError != nil {
			return nil, convertError
		}
		settingsMap[key] = converted
	}
	return settingsMap, nil
}

func (s *ServiceClient) convertUnstructuredListIntoCatalogService(
	unstructuredList *unstructured.UnstructuredList,
) ([]*models.CatalogService, error) {
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

func (s *ServiceClient) convertUnstructuredIntoCatalogService(
	obj unstructured.Unstructured,
) (*models.CatalogService, error) {
	catalogService := apiv1.Service{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		obj.Object,
		&catalogService,
	)

	if err != nil {
		return nil, errors.Wrap(err, "error converting catalog service")
	}

	// Convert from CRD structure to internal model.
	// Description/example are read from unstructured because the vendored
	// application CRD Go types do not expose them yet.
	settings := make(map[string]models.ChartSetting)
	for key, value := range catalogService.Spec.Settings {
		setting := models.ChartSetting{
			Type:    value.Type,
			Minimum: value.Minimum,
			Maximum: value.Maximum,
			Enum:    value.Enum,
		}
		if description, found, _ := unstructured.NestedString(
			obj.Object, "spec", "settings", key, "description",
		); found {
			setting.Description = description
		}
		if example, found, _ := unstructured.NestedString(
			obj.Object, "spec", "settings", key, "example",
		); found {
			setting.Example = example
		}
		settings[key] = setting
	}

	// if a secret was specified try to load the credentials from it
	var repoUsername, repoPassword string
	secretName := catalogService.Spec.HelmRepo.Secret

	if secretName != "" {
		authSecret, secretError := s.kubeClient.GetSecret(
			context.Background(),
			helmchart.Namespace(),
			secretName,
		)

		switch {
		case secretError == nil:
			repoUsername = string(authSecret.Data["username"])
			repoPassword = string(authSecret.Data["password"])

		case k8sapierrors.IsNotFound(secretError):
			// The referenced secret is gone, or was never created. Reading
			// the catalog must not fail for that: a single broken entry
			// would otherwise take the whole listing down with it. Report
			// the entry without credentials instead. Deploying from the
			// repository then fails on its own, with an auth error naming
			// the actual problem.
			logger := helpers.Logger.With("component", "catalog-service")
			logger.Warnw(
				"helm repo auth secret not found, "+
					"continuing without credentials",
				"catalogService", obj.GetName(),
				"secret", secretName,
			)

		default:
			return nil, errors.Wrap(
				secretError,
				"finding helm repo auth secret: "+secretName,
			)
		}
	}

	secretTypes := []string{}
	secretTypesAnnotationValue := catalogService.GetAnnotations()[CatalogServiceSecretTypesAnnotation]
	if len(secretTypesAnnotationValue) > 0 {
		secretTypes = strings.Split(secretTypesAnnotationValue, ",")
	}

	return &models.CatalogService{
		Meta: models.MetaLite{
			Name:      obj.GetName(),
			CreatedAt: obj.GetCreationTimestamp(),
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

func (s *ServiceClient) GetCatalogService(
	ctx context.Context,
	serviceName string,
) (*models.CatalogService, error) {
	result, err := s.serviceKubeClient.Namespace(helmchart.Namespace()).
		Get(ctx, serviceName, metav1.GetOptions{})

	if err != nil {
		return nil, errors.Wrap(
			err,
			fmt.Sprintf(
				"error getting service %s from namespace epinio",
				serviceName,
			),
		)
	}

	service, err := s.convertUnstructuredIntoCatalogService(*result)
	if err != nil {
		return nil, errors.Wrap(err, "error converting result into Catalog Service")
	}

	return service, nil
}

func (s *ServiceClient) ListCatalogServices(
	ctx context.Context,
) ([]*models.CatalogService, error) {
	listResult, err := s.serviceKubeClient.Namespace(helmchart.Namespace()).List(
		ctx,
		metav1.ListOptions{},
	)

	if err != nil {
		return nil, errors.Wrap(err, "error listing services")
	}

	services, err := s.convertUnstructuredListIntoCatalogService(listResult)
	if err != nil {
		return nil, errors.Wrap(
			err,
			"error converting listResult into Catalog Services",
		)
	}

	return services, nil
}
