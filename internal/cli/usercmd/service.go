package usercmd

// ServiceCatalog lists available services
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

	service := catalogShowResponse.Service

	c.ui.Success().WithTable("Key", "Value").
		WithTableRow("Name", service.Name).
		WithTableRow("Short Description", service.ShortDescription).
		WithTableRow("Description", service.Description).
		Msg("Epinio Service:")

	return nil
}
