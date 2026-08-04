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
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

//counterfeiter:generate -header ../../../LICENSE_HEADER . AppchartsService
type AppchartsService interface {
	ChartDefaultSet(ctx context.Context, name string) error
	ChartDefaultShow(ctx context.Context) error
	ChartList(ctx context.Context) error
	ChartShow(ctx context.Context, name string) error
	ChartCreate(ctx context.Context, request models.AppChartCreateRequest) error
	ChartUpdate(ctx context.Context, name string, request models.AppChartUpdateRequest) error
	ChartDelete(ctx context.Context, name string) error

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
		NewAppChartCreateCmd(client),
		NewAppChartUpdateCmd(client),
		NewAppChartDeleteCmd(client),
	)

	return cmd
}

// AppChartWriteConfig holds the flags shared by the create and update commands.
// The update command reuses the same set minus --name.
type AppChartWriteConfig struct {
	name             string
	helmChart        string
	helmRepo         string
	description      string
	shortDescription string
	set              []string
}

// parseChartSet turns the repeatable --set key=value flags into a map. Returns
// nil when no assignments were given, so callers can leave the field untouched.
func parseChartSet(assignments []string) (map[string]string, error) {
	if len(assignments) == 0 {
		return nil, nil
	}

	values := map[string]string{}
	for _, assignment := range assignments {
		pieces := strings.SplitN(assignment, "=", 2)
		if len(pieces) != 2 {
			return nil, errors.New("Bad --set assignment `" + assignment + "`, expected `key=value` as value")
		}
		values[pieces[0]] = pieces[1]
	}

	return values, nil
}

// appChartWriteFlags registers the flags common to create and update (i.e.
// every write flag except --name, which only create carries).
func appChartWriteFlags(cmd *cobra.Command, cfg *AppChartWriteConfig) {
	cmd.Flags().StringVar(&cfg.helmChart, "helm-chart", "", "Helm chart URL")
	cmd.Flags().StringVar(&cfg.helmRepo, "helm-repo", "", "Helm repository URL")
	cmd.Flags().StringVar(&cfg.description, "description", "", "long description")
	cmd.Flags().StringVar(&cfg.shortDescription, "short-description", "", "short description")
	cmd.Flags().StringSliceVar(&cfg.set, "set", []string{}, "values map entry as key=value (repeatable)")
}

// NewAppChartCreateCmd returns a new `epinio app chart create` command
func NewAppChartCreateCmd(client AppchartsService) *cobra.Command {
	cfg := AppChartWriteConfig{}
	cmd := &cobra.Command{
		Use:   "create --name NAME [flags]",
		Short: "Create an application chart",
		Long:  "Create an application chart",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if cfg.name == "" {
				return errors.New("--name is required")
			}

			values, err := parseChartSet(cfg.set)
			if err != nil {
				return err
			}

			request := models.AppChartCreateRequest{
				Name:             cfg.name,
				HelmChart:        cfg.helmChart,
				HelmRepo:         cfg.helmRepo,
				Description:      cfg.description,
				ShortDescription: cfg.shortDescription,
				Values:           values,
			}

			return errors.Wrap(client.ChartCreate(cmd.Context(), request), "error creating app chart")
		},
	}

	cmd.Flags().StringVar(&cfg.name, "name", "", "application chart name (required)")
	appChartWriteFlags(cmd, &cfg)

	return cmd
}

// NewAppChartUpdateCmd returns a new `epinio app chart update` command
func NewAppChartUpdateCmd(client AppchartsService) *cobra.Command {
	cfg := AppChartWriteConfig{}
	cmd := &cobra.Command{
		Use:               "update NAME [flags]",
		Short:             "Update an application chart",
		Long:              "Update an application chart. Unset flags leave the corresponding fields unchanged.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppChartMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			values, err := parseChartSet(cfg.set)
			if err != nil {
				return err
			}

			request := models.AppChartUpdateRequest{
				HelmChart:        cfg.helmChart,
				HelmRepo:         cfg.helmRepo,
				Description:      cfg.description,
				ShortDescription: cfg.shortDescription,
				Values:           values,
			}

			return errors.Wrap(client.ChartUpdate(cmd.Context(), args[0], request), "error updating app chart")
		},
	}

	appChartWriteFlags(cmd, &cfg)

	return cmd
}

// NewAppChartDeleteCmd returns a new `epinio app chart delete` command
func NewAppChartDeleteCmd(client AppchartsService) *cobra.Command {
	return &cobra.Command{
		Use:               "delete NAME",
		Short:             "Delete an application chart",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppChartMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return errors.Wrap(client.ChartDelete(cmd.Context(), args[0]), "error deleting app chart")
		},
	}
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
