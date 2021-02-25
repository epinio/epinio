package services

import (
	"github.com/suse/carrier/internal/interfaces"
)

// CatalogService is a Service created using Service Catalog.
// Implements the Service interface.
type CatalogService struct{}

func CreateCatalogService(name, org, class, plan string, parameters map[string]string) interfaces.Service {
	return &CatalogService{}
}

func (s *CatalogService) Bind() error {
	return nil
}

func (s *CatalogService) Unbind() error {
	return nil
}

func (s *CatalogService) Delete() error {
	return nil
}
