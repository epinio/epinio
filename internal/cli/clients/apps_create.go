package clients

import (
	"encoding/json"

	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/api/v1/models"
)

// AppCreate creates an app without a workload
func (c *EpinioClient) AppCreate(appName string) error {
	log := c.Log.WithName("Apps").WithValues("Organization", c.Config.Org, "Application", appName)
	log.Info("start")
	defer log.Info("return")
	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Organization", c.Config.Org).
		WithStringValue("Application", appName).
		Msg("Create application")

	details.Info("create application")

	request := models.ApplicationCreateRequest{Name: appName}
	b, err := json.Marshal(request)
	if err != nil {
		return nil
	}
	_, err = c.post(api.Routes.Path("AppCreate", c.Config.Org), string(b))
	if err != nil {
		return err
	}

	c.ui.Success().Msg("Ok")
	return nil
}
