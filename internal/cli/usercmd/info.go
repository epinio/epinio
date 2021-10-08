package usercmd

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
		WithStringValue("Epinio Version", v.Version).
		Msg("Epinio Environment")

	return nil
}
