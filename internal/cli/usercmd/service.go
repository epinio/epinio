package usercmd

import "github.com/epinio/epinio/pkg/api/core/v1/models"

// CreateNamespace creates a namespace
func (c *EpinioClient) ServiceCatalog() error {
	log := c.Log.WithName("ServiceCatalog")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Getting catalog...")

	catalog, err := c.API.ServiceCatalog()
	if err != nil {
		return err
	}

	msg := c.ui.Success().WithTable("Name", "Description")

	for _, name := range catalog.Services {
		msg = msg.WithTableRow(
			name.Name,
			name.Description,
		)
	}

	msg.Msg("Epinio Services:")

	return nil
}

// CreateNamespace creates a namespace
func (c *EpinioClient) ServiceCatalogShow(serviceName string) error {
	log := c.Log.WithName("ServiceCatalog")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Getting catalog...")

	catalogShowResponse, err := c.API.ServiceCatalogShow(serviceName)
	if err != nil {
		return err
	}

	service := catalogShowResponse.Service

	c.ui.Success().WithTable("Name", "Description").
		WithTableRow(service.Name, service.Description).
		Msg("Epinio Service:")

	return nil
}

// CreateNamespace creates a namespace
func (c *EpinioClient) ServiceCreate(serviceName, serviceReleaseName string) error {
	log := c.Log.WithName("ServiceCreate")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Creating Service...")

	request := &models.ServiceCreateRequest{
		Name:        serviceName,
		ReleaseName: serviceReleaseName,
	}

	err := c.API.ServiceCreate(request, c.Settings.Namespace)
	if err != nil {
		return err
	}

	// service := catalogShowResponse.Service

	// c.ui.Success().WithTable("Name", "Description").
	// 	WithTableRow(service.Name, service.Description).
	// 	Msg("Epinio Service:")

	return nil
}
