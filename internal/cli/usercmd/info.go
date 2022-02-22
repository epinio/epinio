package usercmd

import (
	"github.com/epinio/epinio/internal/version"
)

// Info displays information about environment
func (c *EpinioClient) Info() error {
	log := c.Log.WithName("Info")
	log.Info("start")
	defer log.Info("return")

	v, err := c.API.Info()
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Platform", v.Platform).
		WithStringValue("Kubernetes Version", v.KubeVersion).
		WithStringValue("Epinio Server Version", v.Version).
		WithStringValue("Epinio Client Version", version.Version).
		Msg("Epinio Environment - I AM AN EVIL CONTRIBUTOR!")

	return nil
}
