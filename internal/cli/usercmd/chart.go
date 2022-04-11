package usercmd

import (
	"context"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// ChartList displays a table of all known application charts.
func (c *EpinioClient) ChartList(ctx context.Context) error {
	log := c.Log.WithName("ChartList")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		Msg("Show Application Charts")

	charts, err := c.API.ChartList()
	if err != nil {
		return err
	}

	msg := c.ui.Success().WithTable("Name", "Description")

	for _, chart := range charts {
		msg = msg.WithTableRow(chart.Name, chart.ShortDescription)
	}

	msg.Msg("Ok")
	return nil
}

// ChartCreate makes a new application chart known to epinio.
func (c *EpinioClient) ChartCreate(ctx context.Context, name, url, repo string) error {
	log := c.Log.WithName("ChartCreate")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Name", name).
		WithStringValue("Repository", repo).
		WithStringValue("Url", url).
		Msg("Create Application Chart")

	request := models.ChartCreateRequest{
		Name:       name,
		Repository: repo,
		URL:        url,
	}

	_, err := c.API.ChartCreate(request)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("OK")
	return nil
}

// ChartShow shows the value of the specified environment variable in
// the named application.
func (c *EpinioClient) ChartShow(ctx context.Context, name string) error {
	log := c.Log.WithName("ChartShow")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		Msg("Show application chart details")

	chart, err := c.API.ChartShow(name)
	if err != nil {
		return err
	}

	c.ui.Success().WithTable("Key", "Value").
		WithTableRow("Name", chart.Name).
		WithTableRow("Short", chart.ShortDescription).
		WithTableRow("Description", chart.Description).
		WithTableRow("Repo", chart.HelmRepo.URL).
		WithTableRow("Url", chart.HelmChart).
		Msg("Details:")

	return nil
}

// ChartDelete removes the named application chart from epinio
func (c *EpinioClient) ChartDelete(ctx context.Context, name string) error {
	log := c.Log.WithName("ChartDelete")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		Msg("Remove application chart")

	_, err := c.API.ChartDelete(name)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("OK")

	return nil
}

// ChartMatching retrieves all application charts in the cluster, for the given prefix
func (c *EpinioClient) ChartMatching(prefix string) []string {
	log := c.Log.WithName("ChartMatching")
	log.Info("start")
	defer log.Info("return")

	resp, err := c.API.ChartMatch(prefix)
	if err != nil {
		// TODO log that we dropped an error
		return []string{}
	}

	return resp.Names
}
