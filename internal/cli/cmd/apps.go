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
	"fmt"
	"os"
	"path/filepath"

	"github.com/epinio/epinio/internal/manifest"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

//counterfeiter:generate -header ../../../LICENSE_HEADER . ApplicationsService
type ApplicationsService interface {
	AppCreate(name string, updateRequest models.ApplicationUpdateRequest) error
	AppDelete(ctx context.Context, appNames []string, all bool) error
	AppExec(ctx context.Context, name, instance string) error
	AppExport(name string, toRegistry bool, exportRequest models.AppExportRequest) error
	AppLogs(name, stageID string, follow bool) error
	AppManifest(name, path string) error
	AppPortForward(ctx context.Context, name, instance string, address, ports []string) error
	AppPush(ctxt context.Context, manifest models.ApplicationManifest) error
	AppRestage(name string, restart bool) error
	AppRestart(name string) error
	AppShow(name string) error
	AppStageID(name string) (string, error)
	AppUpdate(name string, updateRequest models.ApplicationUpdateRequest) error
	Apps(all bool) error

	AppMatcher
	AppChartMatcher
	RegistryMatcher
	ConfigurationMatching(toComplete string) []string // --bind

	// interfaces for the env and chart sub-ensembles
	AppenvService
	AppchartsService
}

// NewApplicationsCmd returns a new 'epinio app' command
func NewApplicationsCmd(client ApplicationsService, rootCfg *RootConfig) *cobra.Command {
	appsCmd := &cobra.Command{
		Use:           "app",
		Aliases:       []string{"apps"},
		Short:         "Epinio application features",
		Long:          `Manage epinio application`,
		SilenceErrors: false,
		Args:          cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Usage(); err != nil {
				return err
			}
			return fmt.Errorf(`Unknown method "%s"`, args[0])
		},
	}

	appsCmd.AddCommand(
		NewAppChartCmd(client), // See appchart.go for implementation
		NewAppCreateCmd(client),
		NewAppDeleteCmd(client),
		NewAppEnvCmd(client), // See appenv.go for implementation
		NewAppExecCmd(client),
		NewAppExportCmd(client),
		NewAppListCmd(client, rootCfg),
		NewAppLogsCmd(client),
		NewAppManifestCmd(client),
		NewAppPortForwardCmd(client),
		NewAppPushCmd(client),
		NewAppRestageCmd(client),
		NewAppRestartCmd(client),
		NewAppShowCmd(client, rootCfg),
		NewAppUpdateCmd(client),
	)

	return appsCmd
}

// NewAppCreateCmd returns a new `epinio apps create` command
func NewAppCreateCmd(client ApplicationsService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create just the app, without creating a workload",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			m, err := manifest.UpdateICE(models.ApplicationManifest{}, cmd)
			if err != nil {
				return errors.Wrap(err, "unable to get app configuration")
			}

			m, err = manifest.UpdateAppChart(m, cmd)
			if err != nil {
				return errors.Wrap(err, "unable to get app chart")
			}

			m, err = manifest.UpdateRoutes(m, cmd)
			if err != nil {
				return err
			}

			updateRequest := models.NewApplicationUpdateRequest(m)
			err = client.AppCreate(args[0], updateRequest)
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error creating app")
		},
	}

	// TODO :: move into cmd args, create config structure - create
	routeOption(cmd)
	bindOption(cmd, client)
	envOption(cmd)
	instancesOption(cmd)
	chartValueOptionX(cmd)

	cmd.Flags().String("app-chart", "", "App chart to use for deployment")
	bindFlag(cmd, "app-chart")
	bindFlagCompletionFunc(cmd, "app-chart", NewAppChartMatcherValueFunc(client))

	return cmd
}

type AppDeleteConfig struct {
	all bool
}

// NewAppDeleteCmd returns a new `epinio apps delete` command
func NewAppDeleteCmd(client ApplicationsService) *cobra.Command {
	cfg := AppDeleteConfig{}
	cmd := &cobra.Command{
		Use:               "delete NAME1 [NAME2 ...]",
		Short:             "Deletes one or more applications",
		ValidArgsFunction: NewAppMatcherAnyFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if cfg.all && len(args) > 0 {
				return errors.New("Conflict between --all and named applications")
			}
			if !cfg.all && len(args) == 0 {
				return errors.New("No applications specified for deletion")
			}

			err := client.AppDelete(cmd.Context(), args, cfg.all)
			if err != nil {
				return errors.Wrap(err, "error deleting app")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&cfg.all, "all", false, "Delete all applications")

	return cmd
}

type AppExecConfig struct {
	instance string
}

// NewAppExecCmd returns a new `epinio apps exec` command
func NewAppExecCmd(client ApplicationsService) *cobra.Command {
	cfg := AppExecConfig{}
	cmd := &cobra.Command{
		Use:               "exec NAME",
		Short:             "creates a shell to the application",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.AppExec(cmd.Context(), args[0], cfg.instance)
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error getting a shell to application")
		},
	}

	cmd.Flags().StringVarP(&cfg.instance, "instance", "i", "",
		"The name of the instance to shell to")

	return cmd
}

type AppExportConfig struct {
	registry     string
	imageName    string
	imageTag     string
	chartName    string
	chartVersion string
}

// NewAppExportCmd return a new `epinio apps export` command
func NewAppExportCmd(client ApplicationsService) *cobra.Command {
	cfg := AppExportConfig{}
	cmd := &cobra.Command{
		Use:               "export NAME [DIRECTORY]",
		Short:             "Export the named application into the directory or flag-specified registry",
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			nargs := len(args)

			// we have to have a single destination. i.e. exactly one of directory or registry
			// has to be specified. specifying neither or both are errors.
			if nargs == 1 && cfg.registry == "" {
				return errors.New("Neither directory nor registry destination found")
			}
			if nargs == 2 && cfg.registry != "" {
				return errors.New("Conflict, both directory and registry destinations found")
			}

			var destination string
			toRegistry := nargs == 1
			if toRegistry {
				destination = cfg.registry
			} else {
				destination = args[1] // directory
			}

			err := client.AppExport(args[0], toRegistry, models.AppExportRequest{
				Destination:  destination,
				ImageName:    cfg.imageName,
				ChartName:    cfg.chartName,
				ImageTag:     cfg.imageTag,
				ChartVersion: cfg.chartVersion,
			})
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error exporting app")
		},
	}

	cmd.Flags().StringVar(&cfg.imageName, "image-name", "", "User chosen name for the image file")
	cmd.Flags().StringVar(&cfg.imageTag, "image-tag", "", "User chosen tag for the image file")
	cmd.Flags().StringVar(&cfg.chartName, "chart-name", "", "User chosen name for the chart file")
	cmd.Flags().StringVar(&cfg.chartVersion, "chart-version", "", "User chosen version for the chart file")

	cmd.Flags().StringVarP(&cfg.registry, "registry", "r", "", "The name of the registry to export to")
	bindFlag(cmd, "registry")
	bindFlagCompletionFunc(cmd, "registry", NewRegistryMatcherValueFunc(client))

	return cmd
}

type AppListConfig struct {
	all bool
}

// NewAppListCmd returns a new `epinio app list` command
func NewAppListCmd(client ApplicationsService, rootCfg *RootConfig) *cobra.Command {
	cfg := AppListConfig{}
	cmd := &cobra.Command{
		Use:   "list [--all]",
		Short: "Lists applications",
		Long:  "Lists applications in the targeted namespace, or all",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.Apps(cfg.all)
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error listing apps")
		},
	}

	cmd.Flags().BoolVar(&cfg.all, "all", false, "list all applications")

	cmd.Flags().VarP(rootCfg.Output, "output", "o", "sets output format [text|json]")
	bindFlag(cmd, "output")
	bindFlagCompletionFunc(cmd, "output", NewStaticFlagsCompletionFunc(rootCfg.Output.Allowed))

	return cmd
}

type AppLogsConfig struct {
	follow  bool
	staging bool
}

// NewAppLogsCmd returns a new `epinio apps logs` command
func NewAppLogsCmd(client ApplicationsService) *cobra.Command {
	cfg := AppLogsConfig{}
	cmd := &cobra.Command{
		Use:               "logs NAME",
		Short:             "Streams the logs of the application",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			stageID := ""
			if cfg.staging {
				stageIDHere, err := client.AppStageID(args[0])
				if err != nil {
					return errors.Wrap(err, "error checking app")
				}
				stageID = stageIDHere
			}

			err := client.AppLogs(args[0], stageID, cfg.follow)
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error streaming application logs")
		},
	}

	cmd.Flags().BoolVar(&cfg.follow, "follow", false, "follow the logs of the application")
	cmd.Flags().BoolVar(&cfg.staging, "staging", false, "show the staging logs of the application")

	return cmd
}

// NewAppManifestCmd returns a new `epinio apps manifest` command
func NewAppManifestCmd(client ApplicationsService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "manifest NAME MANIFESTPATH",
		Short:             "Save state of the named application as a manifest",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.AppManifest(args[0], args[1])
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error getting app manifest")
		},
	}

	return cmd
}

type AppForwardConfig struct {
	address  []string
	instance string
}

// NewAppPortForwardCmd returns a new `epinio apps port-forward` command
func NewAppPortForwardCmd(client ApplicationsService) *cobra.Command {
	cfg := AppForwardConfig{}
	cmd := &cobra.Command{
		Use:               "port-forward NAME [LOCAL_PORT:]REMOTE_PORT [...[LOCAL_PORT_N:]REMOTE_PORT_N]",
		Short:             "forward one or more local ports to a pod",
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			appName := args[0]
			ports := args[1:]

			err := client.AppPortForward(cmd.Context(), appName, cfg.instance, cfg.address, ports)
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error port forwarding to application")
		},
	}

	cmd.Flags().StringSliceVar(&cfg.address, "address", []string{"localhost"},
		"Addresses to listen on (comma separated). Only accepts IP addresses or localhost as a value. When localhost is supplied, kubectl will try to bind on both 127.0.0.1 and ::1 and will fail if neither of these addresses are available to bind.")
	cmd.Flags().StringVarP(&cfg.instance, "instance", "i", "",
		"The name of the instance to shell to")

	return cmd
}

// NewAppPushCmd returns a new `epinio apps push` command
func NewAppPushCmd(client ApplicationsService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push [flags] [PATH_TO_APPLICATION_MANIFEST]",
		Short: "Push an application declared in the specified manifest",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			// Syntax:
			//   - push [flags] [PATH-TO-MANIFEST-FILE]

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

			// Final manifest verify: Name is specified

			if m.Name == "" {
				cmd.SilenceUsage = false
				return errors.New("Name required, not found in manifest nor options")
			}

			// Final completion: Without origin fall back to working directory

			if m.Origin.Kind == models.OriginNone {
				m.Origin.Kind = models.OriginPath
				m.Origin.Path = wd
			}

			if m.Origin.Kind == models.OriginPath {
				if _, err := os.Stat(m.Origin.Path); err != nil {
					// Path issue is user error. Show usage
					cmd.SilenceUsage = false
					return errors.Wrap(err, "path not accessible")
				}
			}

			err = client.AppPush(cmd.Context(), m)
			if err != nil {
				return errors.Wrap(err, "error pushing app to server")
			}

			return nil
		},
	}

	// TODO :: create config structure - push

	// The following options override manifest data
	cmd.Flags().StringP("git", "g", "", "Git repository and revision of sources separated by comma (e.g. GIT_URL,REVISION)")
	cmd.Flags().String("container-image-url", "", "Container image url for the app workload image")
	cmd.Flags().StringP("name", "n", "", "Application name. (mandatory if no manifest is provided)")
	cmd.Flags().StringP("path", "p", "", "Path to application sources.")
	cmd.Flags().String("builder-image", "", "Paketo builder image to use for staging")

	gitProviderOption(cmd)
	routeOption(cmd)
	bindOption(cmd, client)
	envOption(cmd)
	instancesOption(cmd)
	chartValueOptionX(cmd)

	cmd.Flags().String("app-chart", "", "App chart to use for deployment")
	bindFlag(cmd, "app-chart")
	bindFlagCompletionFunc(cmd, "app-chart", NewAppChartMatcherValueFunc(client))

	return cmd
}

type AppRestageConfig struct {
	noRestart bool
}

// NewAppRestageCmd returns a new `epinio app restage` command
func NewAppRestageCmd(client ApplicationsService) *cobra.Command {
	cfg := AppRestageConfig{}
	cmd := &cobra.Command{
		Use:               "restage NAME",
		Short:             "Restage the application, then restart, if running and not suppressed",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			restart := !cfg.noRestart

			err := client.AppRestage(args[0], restart)
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error restaging app")
		},
	}

	cmd.Flags().BoolVar(&cfg.noRestart, "no-restart", false,
		"Do not restart application after restaging")

	return cmd
}

// NewAppRestartCmd returns a new `epinio app restart` command
func NewAppRestartCmd(client ApplicationsService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "restart NAME",
		Short:             "Restart the application",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.AppRestart(args[0])
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error restarting app")
		},
	}

	return cmd
}

// NewAppShowCmd returns a new `epinio apps show` command
func NewAppShowCmd(client ApplicationsService, rootCfg *RootConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "show NAME",
		Short:             "Describe the named application",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.AppShow(args[0])
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error showing app")
		},
	}

	cmd.Flags().VarP(rootCfg.Output, "output", "o", "sets output format [text|json]")
	bindFlag(cmd, "output")
	bindFlagCompletionFunc(cmd, "output", NewStaticFlagsCompletionFunc(rootCfg.Output.Allowed))

	return cmd
}

// NewAppUpdateCmd returns a new `epinio apps update` command
func NewAppUpdateCmd(client ApplicationsService) *cobra.Command {
	// It scales the named app
	cmd := &cobra.Command{
		Use:               "update NAME",
		Short:             "Update the named application",
		Long:              "Update the running application's attributes (e.g. instances)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewAppMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			m, err := manifest.UpdateICE(models.ApplicationManifest{}, cmd)
			if err != nil {
				return errors.Wrap(err, "unable to get app configuration")
			}

			m, err = manifest.UpdateAppChart(m, cmd)
			if err != nil {
				return errors.Wrap(err, "unable to get app chart")
			}

			m, err = manifest.UpdateRoutes(m, cmd)
			if err != nil {
				return errors.Wrap(err, "unable to update domains")
			}

			manifestConfig := m.Configuration
			updateRequest := models.ApplicationUpdateRequest{
				Instances:      manifestConfig.Instances,
				Configurations: manifestConfig.Configurations,
				Environment:    manifestConfig.Environment,
				Routes:         manifestConfig.Routes,
				AppChart:       manifestConfig.AppChart,
				Settings:       manifestConfig.Settings,
			}

			err = client.AppUpdate(args[0], updateRequest)
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error updating the app")
		},
	}

	// TODO :: create config structure - update
	routeOption(cmd)
	bindOption(cmd, client)
	envOption(cmd)
	instancesOption(cmd)
	chartValueOptionX(cmd)

	cmd.Flags().String("app-chart", "", "App chart to use for deployment")
	bindFlag(cmd, "app-chart")
	bindFlagCompletionFunc(cmd, "app-chart", NewAppChartMatcherValueFunc(client))

	return cmd
}
