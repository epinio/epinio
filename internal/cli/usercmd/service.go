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

package usercmd

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/epinio/epinio/helpers/termui"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/kyokomi/emoji"
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

	msg := c.ui.Success().WithTable("Name", "Created", "Version", "Description")

	for _, service := range catalog {
		msg = msg.WithTableRow(
			service.Meta.Name,
			service.Meta.CreatedAt.String(),
			service.AppVersion,
			service.ShortDescription,
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

	catalogService, err := c.API.ServiceCatalogShow(serviceName)
	if err != nil {
		return err
	}

	c.ui.Success().WithTable("Key", "Value").
		WithTableRow("Name", catalogService.Meta.Name).
		WithTableRow("Created", catalogService.Meta.CreatedAt.String()).
		WithTableRow("Version", catalogService.AppVersion).
		WithTableRow("Short Description", catalogService.ShortDescription).
		WithTableRow("Description", catalogService.Description).
		Msg("Epinio Service:")

	return nil
}

// ServiceCreate creates a service
func (c *EpinioClient) ServiceCreate(catalogServiceName, serviceName string, wait bool) error {
	log := c.Log.WithName("ServiceCreate")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Catalog", catalogServiceName).
		WithStringValue("Service", serviceName).
		WithBoolValue("Wait For Completion", wait).
		Msg("Creating Service...")

	request := &models.ServiceCreateRequest{
		CatalogService: catalogServiceName,
		Name:           serviceName,
		Wait:           wait,
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

	service, err := c.API.ServiceShow(request, c.Settings.Namespace)
	if err != nil {
		return errors.Wrap(err, "service show failed")
	}

	if service == nil {
		return errors.New("Service not found")
	}

	boundApps := service.BoundApps
	sort.Strings(boundApps)

	internalRoutes := service.InternalRoutes
	sort.Strings(internalRoutes)

	var msg *termui.Message
	var m string
	if service.ManagedByHelmController {
		msg = c.ui.Exclamation()
		m = "Managed by HelmController. Recreate to remove this dependency"
	} else {
		msg = c.ui.Success()
		m = "Details:"
	}

	msg.WithTable("Key", "Value").
		WithTableRow("Name", service.Meta.Name).
		WithTableRow("Created", service.Meta.CreatedAt.String()).
		WithTableRow("Catalog Service", service.CatalogService).
		WithTableRow("Version", service.CatalogServiceVersion).
		WithTableRow("Status", service.Status.String()).
		WithTableRow("Used-By", strings.Join(boundApps, ", ")).
		WithTableRow("Internal Routes", strings.Join(internalRoutes, ", ")).
		Msg(m)

	return nil
}

// ServiceDelete deletes one or more services, specified by name
func (c *EpinioClient) ServiceDelete(serviceNames []string, unbind, all bool) error {
	if all {
		c.ui.Note().
			WithStringValue("Namespace", c.Settings.Namespace).
			Msg("Querying Services for Deletion...")

		if err := c.TargetOk(); err != nil {
			return err
		}

		// Using the match API with a query matching everything. Avoids transmission
		// of full configuration data and having to filter client-side.
		match, err := c.API.ServiceMatch(c.Settings.Namespace, "")
		if err != nil {
			return err
		}
		if len(match.Names) == 0 {
			c.ui.Exclamation().Msg("No services found to delete")
			return nil
		}

		serviceNames = match.Names
		sort.Strings(serviceNames)
	}

	namesCSV := strings.Join(serviceNames, ", ")
	log := c.Log.WithName("DeleteService").
		WithValues("Services", namesCSV, "Namespace", c.Settings.Namespace)
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Names", namesCSV).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Deleting Services...")

	if !all {
		if err := c.TargetOk(); err != nil {
			return err
		}
	}

	request := models.ServiceDeleteRequest{
		Unbind: unbind,
	}

	var bound []string

	s := c.ui.Progressf("Deleting %s in %s", serviceNames, c.Settings.Namespace)
	defer s.Stop()

	go c.trackDeletion(serviceNames, func() []string {
		match, err := c.API.ServiceMatch(c.Settings.Namespace, "")
		if err != nil {
			return []string{}
		}
		return match.Names
	})

	_, err := c.API.ServiceDelete(request, c.Settings.Namespace, serviceNames,
		func(response *http.Response, bodyBytes []byte, err error) error {
			// nothing special for internal errors and the like
			if response.StatusCode != http.StatusBadRequest {
				return err
			}

			// A bad request happens when the configuration is still bound to one or
			// more applications, and the response contains an array of their names.

			var apiError apierrors.ErrorResponse
			if err := json.Unmarshal(bodyBytes, &apiError); err != nil {
				return err
			}

			bound = strings.Split(apiError.Errors[0].Details, ",")
			return nil
		})

	if err != nil {
		return errors.Wrap(err, "service deletion failed")
	}

	if len(bound) > 0 {
		sort.Strings(bound)
		sort.Strings(bound)
		msg := c.ui.Exclamation().WithTable("Bound Applications")

		for _, app := range bound {
			msg = msg.WithTableRow(app)
		}

		msg.Msg("Unable to delete service. It is still used by")
		c.ui.Exclamation().Compact().Msg("Use --unbind to force the issue")

		return nil
	}

	c.ui.Success().
		WithStringValue("Names", namesCSV).
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Services Removed.")
	return nil
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

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Listing Services...")

	services, err := c.API.ServiceList(c.Settings.Namespace)
	if err != nil {
		return errors.Wrap(err, "service list failed")
	}

	if len(services) == 0 {
		c.ui.Normal().Msg("No services found")
		return nil
	}

	notes := false
	for _, service := range services {
		notes = notes || service.ManagedByHelmController
	}

	sort.Sort(services)

	if notes {
		msg := c.ui.Exclamation().WithTable("", "Name", "Created", "Catalog Service", "Version", "Status", "Applications")
		for _, service := range services {
			note := ""
			if service.ManagedByHelmController {
				note = emoji.Sprintf(":warning: Recreate")
			}

			msg = msg.WithTableRow(
				note,
				service.Meta.Name,
				service.Meta.CreatedAt.String(),
				service.CatalogService,
				service.CatalogServiceVersion,
				service.Status.String(),
				strings.Join(service.BoundApps, ", "),
			)
		}
		msg.Msg("Recreate services managed by HelmController to remove this dependency")
	} else {
		msg := c.ui.Success().WithTable("Name", "Created", "Catalog Service", "Version", "Status", "Applications")
		for _, service := range services {
			msg = msg.WithTableRow(
				service.Meta.Name,
				service.Meta.CreatedAt.String(),
				service.CatalogService,
				service.CatalogServiceVersion,
				service.Status.String(),
				strings.Join(service.BoundApps, ", "),
			)
		}
		msg.Msg("Details:")
	}

	return nil
}

// ServiceListAll list of all the services instances where the user has permissions
func (c *EpinioClient) ServiceListAll() error {
	log := c.Log.WithName("ServiceListAll")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Listing all Services...")

	services, err := c.API.AllServices()
	if err != nil {
		return errors.Wrap(err, "service list failed")
	}

	if len(services) == 0 {
		c.ui.Normal().Msg("No services found")
		return nil
	}

	sort.Sort(services)

	msg := c.ui.Success().WithTable("Namespace", "Name", "Created", "Catalog Service", "Version", "Status", "Application")
	for _, service := range services {
		msg = msg.WithTableRow(
			service.Meta.Namespace,
			service.Meta.Name,
			service.Meta.CreatedAt.String(),
			service.CatalogService,
			service.CatalogServiceVersion,
			service.Status.String(),
			strings.Join(service.BoundApps, ", "),
		)
	}
	msg.Msg("Details:")

	return nil
}

// ServiceMatching returns all Epinio services having the specified prefix in their name
func (c *EpinioClient) ServiceMatching(prefix string) []string {
	log := c.Log.WithName("ServiceMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")

	result := []string{}

	resp, err := c.API.ServiceMatch(c.Settings.Namespace, prefix)
	if err != nil {
		return result
	}

	result = resp.Names

	sort.Strings(result)

	log.Info("matches", "found", result)
	return result
}

// CatalogMatching returns all Epinio catalog entries having the specified prefix in their name
func (c *EpinioClient) CatalogMatching(prefix string) []string {
	log := c.Log.WithName("CatalogMatching").WithValues("PrefixToMatch", prefix)
	log.Info("start")
	defer log.Info("return")

	result := []string{}

	resp, err := c.API.ServiceCatalogMatch(prefix)
	if err != nil {
		return result
	}

	result = resp.Names

	sort.Strings(result)

	log.Info("matches", "found", result)
	return result
}
