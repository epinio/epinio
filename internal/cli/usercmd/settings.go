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

// Package admincmd provides the commands of the admin CLI, which deals with
// installing and configurations
package usercmd

import (
	"context"
	"encoding/pem"
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/fatih/color"

	"github.com/pkg/errors"
)

// SettingsColors will update the settings colors configuration
func (c *EpinioClient) SettingsColors(ctx context.Context, colors bool) error {
	c.ui.Note().
		WithStringValue("Settings", helpers.AbsPath(c.Settings.Location)).
		Msg("Edit Colorization Flag")

	c.Settings.Colors = colors
	if err := c.Settings.Save(); err != nil {
		return err
	}

	c.ui.Success().WithBoolValue("Colors", c.Settings.Colors).Msg("Ok")
	return nil
}

// SettingsShow display the current settings configuration
func (c *EpinioClient) SettingsShow(showPassword, showToken bool) {
	c.ui.Note().
		WithStringValue("Settings", helpers.AbsPath(c.Settings.Location)).
		Msg("Show Settings")

	certInfo := color.CyanString("None defined")
	if c.Settings.Certs != "" {
		certInfo = color.BlueString("Present")
	}

	var password string
	if c.Settings.Password != "" {
		password = "***********"
		if showPassword {
			password = c.Settings.Password
		}
	}

	var token string
	if c.Settings.Token.AccessToken != "" {
		token = "***********"
		if showPassword {
			token = c.Settings.Token.AccessToken
		}
	}

	c.ui.Success().
		WithTable("Key", "Value").
		WithTableRow("Colorized Output", color.MagentaString("%t", c.Settings.Colors)).
		WithTableRow("Current Namespace", color.CyanString(c.Settings.Namespace)).
		WithTableRow("Default App Chart", color.CyanString(c.Settings.AppChart)).
		WithTableRow("API User Name", color.BlueString(c.Settings.User)).
		WithTableRow("API Password", color.BlueString(password)).
		WithTableRow("API Token", color.BlueString(token)).
		WithTableRow("API Url", color.BlueString(c.Settings.API)).
		WithTableRow("WSS Url", color.BlueString(c.Settings.WSS)).
		WithTableRow("Certificates", certInfo).
		Msg("Ok")
}

// SettingsUpdateCA updates the CA credentials stored in the settings
func (c *EpinioClient) SettingsUpdateCA(ctx context.Context) error {
	log := c.Log.WithName("SettingsUpdateCA")
	log.Info("start")
	defer log.Info("return")

	details := log.V(1) // NOTE: Increment of level, not absolute.

	c.ui.Note().
		WithStringValue("Settings", helpers.AbsPath(c.Settings.Location)).
		Msg("Updating CA in the stored credentials from the current cluster")

	if c.Settings.Location == "" {
		return errors.New("settings file not found")
	}

	details.Info("retrieving server locations")

	api := c.Settings.API
	wss := c.Settings.WSS

	details.Info("retrieved server locations", "api", api, "wss", wss)
	details.Info("retrieving certs")

	certs, err := encodeCertificate(api)
	if err != nil {
		c.ui.Exclamation().Msg(err.Error())
		return nil
	}

	details.Info("retrieved certs", "certs", certs)

	c.Settings.Certs = certs

	details.Info("saving",
		"user", c.Settings.User,
		"pass", c.Settings.Password,
		"access_token", c.Settings.Token.AccessToken,
		"api", c.Settings.API,
		"wss", c.Settings.WSS,
		"cert", c.Settings.Certs)

	err = c.Settings.Save()
	if err != nil {
		c.ui.Exclamation().Msg(errors.Wrap(err, "failed to save configuration").Error())
		return nil
	}

	details.Info("saved")

	c.ui.Success().Msg("Ok")
	return nil
}

func encodeCertificate(address string) (string, error) {
	var builder strings.Builder

	cert, err := checkCA(address)
	if err != nil {
		// something bad happened while checking the certificates
		if cert == nil {
			return "", errors.Wrap(err, "error while checking CA")
		}
		// add the untrusted certificate
		pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		builder.Write(pemCert)
	} else {
		// and regularly trusted certs go directly into the result
		// This was missing in PR #1964, and demonstrated as bug with issue #2003
		pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		builder.Write(pemCert)
	}

	return builder.String(), nil
}
