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
	"os"
	"path/filepath"

	"github.com/epinio/epinio/internal/manifest"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

//counterfeiter:generate -header ../../../LICENSE_HEADER . AppBuildService
type AppBuildService interface {
	AppBuild(ctx context.Context, manifest models.ApplicationManifest) error
	BuildList(ctx context.Context) error
	BuildShow(ctx context.Context, name string) error
	BuildDelete(ctx context.Context, name string) error

	AppMatcher
}

// NewAppBuildCmd returns a new 'epinio app build' command.
// Running the parent builds and stores an image (no deploy). Subcommands
// list/show/delete manage existing builds.
func NewAppBuildCmd(client ApplicationsService) *cobra.Command {
	var envReplace bool

	cmd := &cobra.Command{
		Use:   "build [flags] [PATH_TO_APPLICATION_MANIFEST]",
		Short: "Build and store an application image without deploying it",
		Long: `Build and store an application image without deploying it.

Use the same source flags as 'epinio app push' (path, git, manifest). After a
successful build, deploy with: epinio app deploy NAME

Subcommands manage existing builds:
  list, show, delete`,
		Args: cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			wd, err := os.Getwd()
			if err != nil {
				return errors.Wrap(err, "working directory not accessible")
			}

			var manifestPath string
			if len(args) == 1 {
				manifestPath = args[0]
			} else {
				manifestPath = filepath.Join(wd, "epinio.yml")
			}

			m, err := manifest.Get(manifestPath)
			if err != nil {
				cmd.SilenceUsage = false
				return errors.Wrap(err, "Manifest error")
			}

			m, err = manifest.UpdateICE(m, cmd)
			if err != nil {
				return err
			}

			m, err = manifest.UpdateBASN(m, cmd)
			if err != nil {
				return err
			}

			m, err = manifest.UpdateRoutes(m, cmd)
			if err != nil {
				return err
			}

			if m.Name == "" {
				cmd.SilenceUsage = false
				return errors.New("Name required, not found in manifest nor options")
			}

			if m.Origin.Kind == models.OriginNone {
				m.Origin.Kind = models.OriginPath
				m.Origin.Path = wd
			}

			if m.Origin.Kind == models.OriginPath {
				if _, err := os.Stat(m.Origin.Path); err != nil {
					cmd.SilenceUsage = false
					return errors.Wrap(err, "path not accessible")
				}
			}

			if cmd.Flags().Changed("env-replace") {
				m.Configuration.ReplaceEnv = &envReplace
			}

			err = client.AppBuild(cmd.Context(), m)
			if err != nil {
				return errors.Wrap(err, "error building app")
			}

			return nil
		},
	}

	// Same origin flags as push — UpdateBASN/UpdateICE read these by name.
	// Container origins are rejected later in AppBuild (nothing to build).
	cmd.Flags().StringP("git", "g", "", "Git repository and revision of sources separated by comma (e.g. GIT_URL,REVISION)")
	cmd.Flags().String("container-image-url", "", "Container image url (not usable with build; use push/deploy instead)")
	cmd.Flags().StringP("name", "n", "", "Application name. (mandatory if no manifest is provided)")
	cmd.Flags().StringP("path", "p", "", "Path to application sources.")
	cmd.Flags().String("builder-image", "", "Paketo builder image to use for staging")
	cmd.Flags().String("build-mode", "", "Staging build mode: buildpack (default) or dockerfile")
	cmd.Flags().String("dockerfile-path", "", "Path to Dockerfile within the application sources (default: Dockerfile)")

	gitConfigOption(cmd, client)
	routeOption(cmd)
	bindOption(cmd, client)
	envOption(cmd)
	instancesOption(cmd)
	chartValueOptionX(cmd)
	cmd.Flags().BoolVar(&envReplace, "env-replace", false, "Replace existing environment instead of merging")
	bindFlag(cmd, "env-replace")

	cmd.Flags().String("app-chart", "", "App chart to use for deployment")
	bindFlag(cmd, "app-chart")
	bindFlagCompletionFunc(cmd, "app-chart", NewAppChartMatcherValueFunc(client))

	cmd.AddCommand(
		NewAppBuildListCmd(client),
		NewAppBuildShowCmd(client),
		NewAppBuildDeleteCmd(client),
	)

	return cmd
}

// NewAppBuildListCmd returns a new `epinio app build list` command
func NewAppBuildListCmd(client AppBuildService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List applications and their current build",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return errors.Wrap(client.BuildList(cmd.Context()), "error listing app builds")
		},
	}

	return cmd
}

// NewAppBuildShowCmd returns a new `epinio app build show` command
func NewAppBuildShowCmd(client AppBuildService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "show NAME",
		Short:             "Show the named application's current build",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return errors.Wrap(client.BuildShow(cmd.Context(), args[0]), "error showing app build")
		},
	}

	return cmd
}

// NewAppBuildDeleteCmd returns a new `epinio app build delete` command
func NewAppBuildDeleteCmd(client AppBuildService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "delete NAME",
		Short:             "Delete the named application's current build",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			return errors.Wrap(client.BuildDelete(cmd.Context(), args[0]), "error deleting app build")
		},
	}

	return cmd
}
