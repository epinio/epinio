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

	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// EnvList displays a table of all environment variables and their
// values for the named application, separated by origin (user vs service-provided).
func (c *EpinioClient) EnvList(ctx context.Context, appName string) error {
	log := c.Log.WithName("EnvList")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		Msg("Show Application Environment")

	if err := c.TargetOk(); err != nil {
		return err
	}

	groupedEnv, err := c.API.EnvListGrouped(c.Settings.Namespace, appName)
	if err != nil {
		return err
	}

	// Display user-provided environment variables
	if len(groupedEnv.User) > 0 {
		msg := c.ui.Success().WithTable("Variable", "Value")
		for _, ev := range groupedEnv.User.List() {
			msg = msg.WithTableRow(ev.Name, ev.Value)
		}
		msg.Msg("User-provided Environment Variables:")
	} else {
		c.ui.Note().Msg("No user-provided environment variables")
	}

	// Display service-provided environment variables
	if len(groupedEnv.Service) > 0 {
		msg := c.ui.Success().WithTable("Variable", "Value")
		for _, ev := range groupedEnv.Service.List() {
			msg = msg.WithTableRow(ev.Name, ev.Value)
		}
		msg.Msg("Service-provided Environment Variables:")
	} else {
		c.ui.Note().Msg("No service-provided environment variables")
	}

	return nil
}

// EnvSet adds or modifies the specified environment variable in the
// named application, with the given value. A workload is restarted.
func (c *EpinioClient) EnvSet(ctx context.Context, appName, envName, envValue string) error {
	log := c.Log.WithName("Env")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		WithStringValue("Variable", envName).
		WithStringValue("Value", envValue).
		Msg("Extend or modify application environment")

	if err := c.TargetOk(); err != nil {
		return err
	}

	request := models.EnvVariableMap{}
	request[envName] = envValue

	_, err := c.API.EnvSet(request, c.Settings.Namespace, appName)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("OK")
	return nil
}

// EnvShow shows the value of the specified environment variable in
// the named application.
func (c *EpinioClient) EnvShow(ctx context.Context, appName, envName string) error {
	log := c.Log.WithName("Env")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		WithStringValue("Variable", envName).
		Msg("Show application environment variable")

	if err := c.TargetOk(); err != nil {
		return err
	}

	eVariable, err := c.API.EnvShow(c.Settings.Namespace, appName, envName)
	if err != nil {
		return err
	}

	c.ui.Success().
		WithStringValue("Value", eVariable.Value).
		Msg("OK")

	return nil
}

// EnvUnset removes the specified environment variable from the named
// application. A workload is restarted.
func (c *EpinioClient) EnvUnset(ctx context.Context, appName, envName string) error {
	log := c.Log.WithName("Env")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().
		WithStringValue("Namespace", c.Settings.Namespace).
		WithStringValue("Application", appName).
		WithStringValue("Variable", envName).
		Msg("Remove from application environment")

	if err := c.TargetOk(); err != nil {
		return err
	}

	_, err := c.API.EnvUnset(c.Settings.Namespace, appName, envName)
	if err != nil {
		return err
	}

	c.ui.Success().Msg("OK")

	return nil
}

// EnvMatching retrieves all environment variables in the cluster, for
// the specified application, and the given prefix
func (c *EpinioClient) EnvMatching(ctx context.Context, appName, prefix string) []string {
	log := c.Log.WithName("Env")
	log.Info("start")
	defer log.Info("return")

	resp, err := c.API.EnvMatch(c.Settings.Namespace, appName, prefix)
	if err != nil {
		// TODO log that we dropped an error
		return []string{}
	}

	return resp.Names
}
