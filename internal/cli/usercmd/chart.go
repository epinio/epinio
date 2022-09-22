package usercmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/fatih/color"
	"github.com/pkg/errors"
)

// ChartDefaultSet sets the local app chart default
func (c *EpinioClient) ChartDefaultSet(ctx context.Context, chartName string) error {
	log := c.Log.WithName("ChartDefaultSet")
	log.Info("start")
	defer log.Info("return")

	// Validate chosen app chart to exist
	if chartName != "" {
		_, err := c.API.ChartShow(chartName)
		if err != nil {
			return err
		}
	}

	// Save to settings
	c.Settings.AppChart = chartName
	err := c.Settings.Save()
	if err != nil {
		return errors.Wrap(err, "failed to save settings")
	}

	// And report
	if chartName == "" {
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

	msg := c.ui.Success().WithTable("Default", "Name", "Created", "Description", "#Settings")

	for _, chart := range charts {
		mark := ""
		name := chart.Meta.Name
		created := chart.Meta.CreatedAt.String()
		short := chart.ShortDescription
		numSettings := fmt.Sprintf(`%d`, len(chart.Settings))

		if chart.Meta.Name == c.Settings.AppChart {
			mark = color.BlueString("*")
			name = color.BlueString(name)
			created = color.BlueString(created)
			short = color.BlueString(short)
			numSettings = color.BlueString(numSettings)
		}
		msg = msg.WithTableRow(mark, name, created, short, numSettings)
	}

	msg.Msg("Ok")
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

	c.ui.Note().WithTable("Key", "Value").
		WithTableRow("Name", chart.Meta.Name).
		WithTableRow("Created", chart.Meta.CreatedAt.String()).
		WithTableRow("Short", chart.ShortDescription).
		WithTableRow("Description", chart.Description).
		WithTableRow("Helm Repository", chart.HelmRepo).
		WithTableRow("Helm Chart", chart.HelmChart).
		Msg("Details:")

	if len(chart.Settings) > 0 {
		var keys []string
		for key := range chart.Settings {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		msg := c.ui.Note().WithTable("Key", "Type", "Allowed Values")

		for _, key := range keys {
			spec := chart.Settings[key]
			msg = msg.WithTableRow(key, spec.Type, details(spec))
		}

		msg.Msg("Settings")
	} else {
		c.ui.Exclamation().Msg("No settings")
	}

	c.ui.Success().Msg("Ok")

	return nil
}

func details(spec models.AppChartSetting) string {
	// Type expected to be in (string, bool, number, integer)
	if spec.Type == "bool" {
		return ""
	}
	if spec.Type == "string" {
		if len(spec.Enum) > 0 {
			return strings.Join(spec.Enum, ", ")
		}
		return ""
	}
	if spec.Type == "number" || spec.Type == "integer" {
		if len(spec.Enum) > 0 {
			return strings.Join(spec.Enum, ", ")
		}
		if spec.Minimum != "" || spec.Maximum != "" {
			min := "-inf"
			if spec.Minimum != "" {
				min = spec.Minimum
			}
			max := "+inf"
			if spec.Maximum != "" {
				max = spec.Maximum
			}
			return fmt.Sprintf(`[%s ... %s]`, min, max)
		}
		return ""
	}
	return "<unknown type>"
}

// ChartMatching retrieves all application charts in the cluster, for the given prefix
func (c *EpinioClient) ChartMatching(prefix string) []string {
	log := c.Log.WithName("ChartMatching")
	log.Info("start")
	defer log.Info("return")

	resp, err := c.API.ChartMatch(prefix)
	if err != nil {
		log.Error(err, "calling chart match endpoint")
		return []string{}
	}

	return resp.Names
}
