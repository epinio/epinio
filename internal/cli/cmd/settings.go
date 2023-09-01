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

package cmd

import (
	"fmt"
	"strconv"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NewSettingsCmd returns a new 'epinio settings' command
func NewSettingsCmd(client *usercmd.EpinioClient) *cobra.Command {
	settingsCmd := &cobra.Command{
		Use:           "settings",
		Short:         "Epinio settings management",
		Long:          `Manage the epinio cli settings`,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Usage(); err != nil {
				return err
			}
			return fmt.Errorf(`Unknown method "%s"`, args[0])
		},
	}

	settingsCmd.AddCommand(
		NewSettingsColorsCmd(client),
		NewSettingsShowCmd(client),
		NewSettingsUpdateCACmd(client),
	)

	return settingsCmd
}

// NewSettingsColorsCmd returns a new 'epinio settings colors' command
func NewSettingsColorsCmd(client *usercmd.EpinioClient) *cobra.Command {
	return &cobra.Command{
		Use:          "colors BOOL",
		Short:        "Manage colored output",
		Long:         "Enable/Disable colored output",
		SilenceUsage: true,
		Args: func(cmd *cobra.Command, args []string) error {
			err := cobra.ExactArgs(1)(cmd, args)
			if err != nil {
				return err
			}

			_, err = strconv.ParseBool(args[0])
			if err != nil {
				return errors.New("requires a boolean argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			colors, err := strconv.ParseBool(args[0])
			// assert: err == nil -- see args validation
			if err != nil {
				return errors.Wrap(err, "unexpected bool parsing error")
			}

			err = client.SettingsColors(cmd.Context(), colors)
			if err != nil {
				return errors.Wrap(err, "setting color")
			}
			return nil
		},
	}
}

type SettingsShowConfig struct {
	showPassword bool
	showToken    bool
}

// NewSettingsShowCmd returns a new 'epinio settings show' command
func NewSettingsShowCmd(client *usercmd.EpinioClient) *cobra.Command {
	cfg := &SettingsShowConfig{}

	settingsShowCmd := &cobra.Command{
		Use:          "show",
		Short:        "Show the current settings",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			showPassword := cfg.showPassword
			showToken := cfg.showPassword

			client.SettingsShow(showPassword, showToken)
		},
	}

	settingsShowCmd.Flags().BoolVar(&cfg.showPassword, "show-password", false, "Show hidden password")
	settingsShowCmd.Flags().BoolVar(&cfg.showToken, "show-token", false, "Show access token")

	bindFlag(settingsShowCmd, "show-password")
	bindFlag(settingsShowCmd, "show-token")

	return settingsShowCmd
}

// NewSettingsUpdateCACmd returns a new 'epinio settings update-ca' command
func NewSettingsUpdateCACmd(client *usercmd.EpinioClient) *cobra.Command {
	return &cobra.Command{
		Use:          "update-ca",
		Short:        "Update the api location and CA certificate",
		Long:         "Update the api location and CA certificate from the current cluster",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := client.SettingsUpdateCA(cmd.Context())
			if err != nil {
				return errors.Wrap(err, "failed to update the settings")
			}
			return nil
		},
	}
}
