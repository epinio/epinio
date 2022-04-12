package usercmd

import (
	"context"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/fatih/color"
	"github.com/pkg/errors"
)

// ChartDefaultSet sets the local app chart default
func (c *EpinioClient) ChartDefaultSet(ctx context.Context, chart string) error {
	log := c.Log.WithName("ChartDefaultSet")
	log.Info("start")
	defer log.Info("return")

	c.Settings.AppChart = chart
	err := c.Settings.Save()
	if err != nil {
		return errors.Wrap(err, "failed to save settings")
	}

	if chart == "" {
		c.ui.Note().
			Msg("Unset Default Application Chart")
	} else {
		c.ui.Note().
			WithStringValue("Name", c.Settings.AppChart).
			Msg("New Default Application Chart")
	}

	return nil
}

// ChartDefaultShow displays the current local app chart default
func (c *EpinioClient) ChartDefaultShow(ctx context.Context) error {
	log := c.Log.WithName("ChartDefaultShow")
	log.Info("start")
	defer log.Info("return")

	chart := c.Settings.AppChart
	if chart == "" {
		chart = color.CyanString("not set, system default applies")
	}

	c.ui.Note().
		WithStringValue("Name", chart).
		Msg("Default Application Chart")

	return nil
}

// ChartList displays a table of all known application charts.
func (c *EpinioClient) ChartList(ctx context.Context) error {
	log := c.Log.WithName("ChartList")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		Msg("Show Application Charts")

	charts, err := c.API.ChartList()
	if err != nil {
		return err
	}

	msg := c.ui.Success().WithTable("Default", "Name", "Description")

	for _, chart := range charts {
		mark := ""
		name := chart.Name
		short := chart.ShortDescription
		if chart.Name == c.Settings.AppChart {
			mark = color.BlueString("*")
			name = color.BlueString(name)
			short = color.BlueString(short)
		}
		msg = msg.WithTableRow(mark, name, short)
	}

	msg.Msg("Ok")
	return nil
}

// ChartCreate makes a new application chart known to epinio.
func (c *EpinioClient) ChartCreate(ctx context.Context, name, chart, short, desc, repo string) error {
	log := c.Log.WithName("ChartCreate")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		WithStringValue("Short Description", short).
		WithStringValue("Description", desc).
		WithStringValue("Helm Chart", chart).
		WithStringValue("Helm Repository", repo).
		Msg("Create Application Chart")

	request := models.ChartCreateRequest{
		Name:        name,
		ShortDesc:   short,
		Description: desc,
		HelmRepo:    repo,
		HelmChart:   chart,
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
		WithTableRow("Helm Repository", chart.HelmRepo).
		WithTableRow("Helm Chart", chart.HelmChart).
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
