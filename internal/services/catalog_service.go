package services

import (
	"github.com/suse/carrier/internal/interfaces"
	corev1 "k8s.io/api/core/v1"
)

// CatalogService is a Service created using Service Catalog.
// Implements the Service interface.
type CatalogService struct{}

func CreateCatalogService(name, org, class, plan string, parameters map[string]string) interfaces.Service {
	// TODO: create catalog service via service catalog (service binding resource)
	// TODO: Create parallel secret for easier listing (no special cases in the generic code)
	return &CatalogService{}
}

func (s *CatalogService) Name() string {
	// TODO return name of catalog service
	return ""
}

func (s *CatalogService) Org() string {
	// TODO return org of catalog service
	return ""
}

// GetBinding returns an application-specific secret for the service to be
// bound to that application.
func (s *CatalogService) GetBinding(appName string) (*corev1.Secret, error) {
	// TODO bind catalog service to app
	// - Create ServiceBinding resource
	// - Wait for service catalog to create the secret
	// - Label the secret
	// - Return secret

	return nil, nil
}

func (s *CatalogService) Delete() error {
	// TODO delete catalog service via service catalog
	return nil
}
