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
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

//counterfeiter:generate -header ../../../LICENSE_HEADER . AppenvService
type AppenvService interface {
	EnvList(ctx context.Context, appname string) error
	EnvSet(ctx context.Context, appname, name, value string) error
	EnvShow(ctx context.Context, appname, name string) error
	EnvUnset(ctx context.Context, appname, name string) error

	AppMatcher
	AppVarMatcher
}

// NewAppEnvCmd returns a new 'epinio app chart' command
func NewAppEnvCmd(client AppenvService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Epinio application configuration",
		Long:  `Manage epinio application environment variables`,
	}

	cmd.AddCommand(
		NewAppEnvListCmd(client),
		NewAppEnvSetCmd(client),
		NewAppEnvShowCmd(client),
		NewAppEnvUnsetCmd(client),
	)

	return cmd
}

// NewAppEnvListCmd returns a new `epinio app env list` command
func NewAppEnvListCmd(client AppenvService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "list APPNAME",
		Short:             "Lists application environment",
		Long:              "Lists environment variables of named application",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.EnvList(cmd.Context(), args[0])
			if err != nil {
				return errors.Wrap(err, "error listing app environment")
			}

			return nil
		},
	}

	return cmd
}

// NewAppEnvSetCmd returns a new `epinio app env set` command
func NewAppEnvSetCmd(client AppenvService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "set APPNAME NAME VALUE",
		Short:             "Extend application environment",
		Long:              "Add or change environment variable of named application",
		Args:              cobra.ExactArgs(3),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.EnvSet(cmd.Context(), args[0], args[1], args[2])
			if err != nil {
				return errors.Wrap(err, "error setting into app environment")
			}

			return nil
		},
	}

	return cmd
}

// NewAppEnvShowCmd returns a new `epinio app env show` command
func NewAppEnvShowCmd(client AppenvService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "show APPNAME NAME",
		Short:             "Describe application's environment variable",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: NewAppVarMatcherFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.EnvShow(cmd.Context(), args[0], args[1])
			if err != nil {
				return errors.Wrap(err, "error accessing app environment")
			}

			return nil
		},
	}

	return cmd
}

// NewAppEnvUnsetCmd returns a new `epinio app env unset` command
func NewAppEnvUnsetCmd(client AppenvService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "unset APPNAME NAME",
		Short:             "Shrink application environment",
		Long:              "Remove environment variable from named application",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: NewAppVarMatcherFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.EnvUnset(cmd.Context(), args[0], args[1])
			if err != nil {
				return errors.Wrap(err, "error removing from app environment")
			}

			return nil
		},
	}

	return cmd
}
