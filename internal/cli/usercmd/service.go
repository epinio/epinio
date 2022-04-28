package usercmd

import (
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
)

// ServiceCatalog lists available services
func (c *EpinioClient) ServiceCatalog() error {
	log := c.Log.WithName("ServiceCatalog")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Getting catalog...")

	catalog, err := c.API.ServiceCatalog()
	if err != nil {
		return errors.Wrap(err, "service catalog failed")
	}

	msg := c.ui.Success().WithTable("Name", "Description")

	for _, name := range catalog.CatalogServices {
		msg = msg.WithTableRow(
			name.Name,
			name.ShortDescription,
		)
	}

	msg.Msg("Epinio Services:")

	return nil
}

// ServiceCatalogShow shows a service
func (c *EpinioClient) ServiceCatalogShow(serviceName string) error {
	log := c.Log.WithName("ServiceCatalog")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Service", serviceName).
		Msg("Show service details")

	catalogShowResponse, err := c.API.ServiceCatalogShow(serviceName)
	if err != nil {
		return err
	}

	service := catalogShowResponse.CatalogService

	c.ui.Success().WithTable("Key", "Value").
		WithTableRow("Name", service.Name).
		WithTableRow("Short Description", service.ShortDescription).
		WithTableRow("Description", service.Description).
		Msg("Epinio Service:")

	return nil
}

// ServiceCreate creates a service
func (c *EpinioClient) ServiceCreate(catalogServiceName, serviceName string) error {
	log := c.Log.WithName("ServiceCreate")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Creating Service...")

	request := &models.ServiceCreateRequest{
		CatalogService: catalogServiceName,
		Name:           serviceName,
	}

	err := c.API.ServiceCreate(request, c.Settings.Namespace)
	// Note: errors.Wrap (nil, "...") == nil
	return errors.Wrap(err, "service create failed")
}

// ServiceShow describes a service instance
func (c *EpinioClient) ServiceShow(serviceName string) error {
	log := c.Log.WithName("ServiceShow")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Showing Service...")

	request := &models.ServiceShowRequest{
		Name: serviceName,
	}

	resp, err := c.API.ServiceShow(request, c.Settings.Namespace)
	if err != nil {
		return errors.Wrap(err, "service show failed")
	}

	if resp.Service == nil {
		return errors.New("Service not found")
	}

	c.ui.Success().WithTable("Key", "Value").
		WithTableRow("Name", resp.Service.Name).
		WithTableRow("Catalog Service", resp.Service.CatalogService).
		WithTableRow("Status", resp.Service.Status.String()).
		Msg("Details:")

	return nil
}

// ServiceDelete deletes a service
func (c *EpinioClient) ServiceDelete(name string) error {
	log := c.Log.WithName("ServiceDelete")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Deleting Service...")

	err := c.API.ServiceDelete(c.Settings.Namespace, name)

	return errors.Wrap(err, "service deletion failed")
}

// ServiceBind binds a service to an application
func (c *EpinioClient) ServiceBind(name, appName string) error {
	log := c.Log.WithName("ServiceBind")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Binding Service...")

	request := &models.ServiceBindRequest{
		AppName: appName,
	}

	err := c.API.ServiceBind(request, c.Settings.Namespace, name)
	// Note: errors.Wrap (nil, "...") == nil
	return errors.Wrap(err, "service bind failed")
}

// ServiceUnbind unbinds a service from an application
func (c *EpinioClient) ServiceUnbind(name, appName string) error {
	log := c.Log.WithName("ServiceUnbind")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Unbinding Service...")

	request := &models.ServiceUnbindRequest{
		AppName: appName,
	}

	err := c.API.ServiceUnbind(request, c.Settings.Namespace, name)
	return errors.Wrap(err, "service unbind failed")
}

// ServiceList list of the service instances in the targeted namespace
func (c *EpinioClient) ServiceList() error {
	log := c.Log.WithName("ServiceList")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Listing Services...")

	resp, err := c.API.ServiceList(c.Settings.Namespace)
	if err != nil {
		return errors.Wrap(err, "service list failed")
	}

	if len(resp.Services) == 0 {
		c.ui.Normal().Msg("No services found")
		return nil
	}

	msg := c.ui.Success().WithTable("Name", "Catalog Service", "Status")
	for _, service := range resp.Services {
		msg = msg.WithTableRow(service.Name, service.CatalogService, service.Status.String())
	}
	msg.Msg("Details:")

	return nil
}
