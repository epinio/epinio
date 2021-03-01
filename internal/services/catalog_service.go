package services

import (
	"github.com/suse/carrier/internal/interfaces"
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

func (s *CatalogService) Bind(app interfaces.Application) error {
	// TODO bind catalog service to app
	return nil
}

func (s *CatalogService) Unbind(app interfaces.Application) error {
	// TODO remove catalog service binding to app
	return nil
}

func (s *CatalogService) Delete() error {
	// TODO delete catalog service via service catalog
	return nil
}
