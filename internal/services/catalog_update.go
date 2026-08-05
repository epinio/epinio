package services

import (
	"context"
	"strings"

	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
