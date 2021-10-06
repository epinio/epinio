package usercmd

import (
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// AppCreate creates an app without a workload
func (c *EpinioClient) AppCreate(appName string, appConfig models.ApplicationUpdateRequest) error {
	log := c.Log.WithName("Apps").WithValues("Organization", c.Config.Org, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Organization", c.Config.Org).
		WithStringValue("Application", appName).
		Msg("Create application")

	details.Info("create application")

	request := models.ApplicationCreateRequest{
		Name:          appName,
		Configuration: appConfig,
	}

	_, err := c.API.AppCreate(request, c.Config.Org)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Ok")
	return nil
}
