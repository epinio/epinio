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
	"context"
	"fmt"

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
		created := formatCreatedAt(chart.Meta.CreatedAt)
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
		WithTableRow("Created", formatCreatedAt(chart.Meta.CreatedAt)).
		WithTableRow("Short", chart.ShortDescription).
		WithTableRow("Description", chart.Description).
		WithTableRow("Helm Repository", chart.HelmRepo).
		WithTableRow("Helm Chart", chart.HelmChart).
		Msg("Details:")

	c.ChartSettingsShow(ctx, chart.Settings)

	c.ui.Success().Msg("Ok")

	return nil
}

// ChartCreate creates an application chart from the supplied request.
func (c *EpinioClient) ChartCreate(ctx context.Context, request models.AppChartCreateRequest) error {
	log := c.Log.WithName("ChartCreate")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", request.Name).
		Msg("Creating Application Chart...")

	_, err := c.API.ChartCreate(request)
	if err != nil {
		return errors.Wrap(err, "chart create failed")
	}

	c.ui.Success().
		WithStringValue("Name", request.Name).
		Msg("Application Chart Created.")

	return nil
}

// ChartUpdate updates the named application chart. Omitted request fields leave
// the corresponding values unchanged on the server.
func (c *EpinioClient) ChartUpdate(ctx context.Context, name string, request models.AppChartUpdateRequest) error {
	log := c.Log.WithName("ChartUpdate")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		Msg("Updating Application Chart...")

	_, err := c.API.ChartUpdate(name, request)
	if err != nil {
		return errors.Wrap(err, "chart update failed")
	}

	c.ui.Success().
		WithStringValue("Name", name).
		Msg("Application Chart Updated.")

	return nil
}

// ChartDelete deletes the named application chart.
func (c *EpinioClient) ChartDelete(ctx context.Context, name string) error {
	log := c.Log.WithName("ChartDelete")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Name", name).
		Msg("Deleting Application Chart...")

	_, err := c.API.ChartDelete(name)
	if err != nil {
		return errors.Wrap(err, "chart delete failed")
	}

	c.ui.Success().
		WithStringValue("Name", name).
		Msg("Application Chart Removed.")

	return nil
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
