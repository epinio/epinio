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

	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/pkg/errors"
)

// Login removes all authentication information from the settings file
func (c *EpinioClient) Logout(ctx context.Context) error {
	var err error

	log := c.Log.WithName("Logout")
	log.Info("start")
	defer log.Info("return")

	c.ui.Note().Msg("Logout from Epinio clusters")

	// load settings and update them (in memory)
	updatedSettings, err := clearAuthSettings()
	if err != nil {
		return errors.Wrap(err, "error updating settings")
	}

	c.ui.Success().Msg("Logout successful")

	err = updatedSettings.Save()
	return errors.Wrap(err, "error saving new settings")
}

func clearAuthSettings() (*settings.Settings, error) {
	epinioSettings, err := settings.Load()
	if err != nil {
		return nil, errors.Wrap(err, "error loading the settings")
	}

	epinioSettings.API = ""
	epinioSettings.WSS = ""
	epinioSettings.User = ""
	epinioSettings.Password = ""
	epinioSettings.Certs = ""
	epinioSettings.Token = settings.TokenSetting{}

	return epinioSettings, nil
}
