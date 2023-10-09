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

//counterfeiter:generate -header ../../../LICENSE_HEADER . AppchartsService
type AppchartsService interface {
	ChartDefaultSet(ctx context.Context, name string) error
	ChartDefaultShow(ctx context.Context) error
	ChartList(ctx context.Context) error
	ChartShow(ctx context.Context, name string) error

	AppChartMatcher
}

// NewAppchartsCmd returns a new 'epinio app chart' command
func NewAppChartCmd(client AppchartsService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chart",
		Short: "Epinio application chart management",
		Long:  `Manage epinio application charts`,
	}

	cmd.AddCommand(
		NewAppChartDefaultCmd(client),
		NewAppChartListCmd(client),
		NewAppChartShowCmd(client),
	)

	return cmd
}

// NewAppChartDefaultCmd returns a new `epinio app chart default` command
func NewAppChartDefaultCmd(client AppchartsService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "default [CHARTNAME]",
		Short:             "Set or show app chart default",
		Long:              "Set or show app chart default",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: NewAppChartMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if len(args) == 1 {
				err := client.ChartDefaultSet(cmd.Context(), args[0])
				if err != nil {
					return errors.Wrap(err, "error setting app chart default")
				}
			} else {
				err := client.ChartDefaultShow(cmd.Context())
				if err != nil {
					return errors.Wrap(err, "error showing app chart default")
				}
			}

			return nil
		},
	}

	return cmd
}

// NewAppChartListCmd returns a new `epinio app chart list` command
func NewAppChartListCmd(client AppchartsService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List application charts",
		Long:  "List applications charts",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.ChartList(cmd.Context())
			if err != nil {
				return errors.Wrap(err, "error listing app charts")
			}

			return nil
		},
	}

	return cmd
}

// NewAppChartShowCmd returns a new `epinio app env show` command
func NewAppChartShowCmd(client AppchartsService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "show CHARTNAME",
		Short:             "Describe application chart",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppChartMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.ChartShow(cmd.Context(), args[0])
			if err != nil {
				return errors.Wrap(err, "error showing app chart")
			}

			return nil
		},
	}

	return cmd
}
