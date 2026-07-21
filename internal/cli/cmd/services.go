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
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

//counterfeiter:generate -header ../../../LICENSE_HEADER . ServicesService
type ServicesService interface {
	ServiceBind(serviceName, appName string) error
	ServiceBatchBind(appName string, serviceNames []string) error
	ServiceCatalog() error
	ServiceCatalogShow(ctx context.Context, serviceName string) error
	ServiceCatalogCreate(request models.CatalogServiceCreateRequest) error
	ServiceCatalogUpdate(name string, request models.CatalogServiceUpdateRequest) error
	ServiceCatalogDelete(name string) error
	ServiceCreate(catalogName, serviceName string, wait bool, chartValues models.ChartValueSettings) error
	ServiceDelete(serviceNames []string, unbind, all bool) error
	ServiceList() error
	ServiceListAll() error
	ServicePortForward(ctx context.Context, serviceName string, address, ports []string) error
	ServiceShow(serviceName string) error
	ServiceUnbind(serviceName, appName string) error
	ServiceUpdate(serviceName string, wait bool, removed []string, assignments map[string]string, noRestart bool) error

	ServiceMatcher
	ServiceChartValueMatcher
	ServiceAppMatcher

	CatalogMatching(toComplete string) []string
}

// NewServicesCmd returns a new 'epinio services' command
func NewServicesCmd(client ServicesService, rootCfg *RootConfig) *cobra.Command {
	servicesCmd := &cobra.Command{
		Use:           "service",
		Aliases:       []string{"services"},
		Short:         "Epinio service management",
		Long:          `Manage the epinio services`,
		SilenceErrors: false,
		Args:          cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Usage(); err != nil {
				return err
			}
			return fmt.Errorf(`unknown method "%s"`, args[0])
		},
	}

	servicesCmd.AddCommand(
		NewServiceBindCmd(client),
		NewServiceCatalogCmd(client),
		NewServiceCreateCmd(client),
		NewServiceDeleteCmd(client),
		NewServiceListCmd(client, rootCfg),
		NewServicePortForwardCmd(client),
		NewServiceShowCmd(client, rootCfg),
		NewServiceUnbindCmd(client),
		NewServiceUpdateCmd(client),
	)

	return servicesCmd
}

// NewServiceCatalogCmd returns a new `epinio service catalog` command. With no
// argument it lists the catalog; with a NAME it shows that entry. The create,
// update, and delete subcommands manage catalog entries.
func NewServiceCatalogCmd(client ServicesService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "catalog [NAME]",
		Short:             "Lists all available Epinio catalog services, or show the details of the specified one",
		ValidArgsFunction: FirstArgValidator(client.CatalogMatching),
		Args:              cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if len(args) == 0 {
				err := client.ServiceCatalog()
				return errors.Wrap(err, "error listing Epinio catalog services")
			}

			if len(args) == 1 {
				serviceName := args[0]
				err := client.ServiceCatalogShow(cmd.Context(), serviceName)
				return errors.Wrap(err, fmt.Sprintf("error showing %s Epinio catalog service", serviceName))
			}

			return nil
		},
	}

	cmd.AddCommand(
		NewServiceCatalogCreateCmd(client),
		NewServiceCatalogUpdateCmd(client),
		NewServiceCatalogDeleteCmd(client),
	)

	return cmd
}

// CatalogWriteConfig holds the flags shared by the catalog create and update
// commands. The update command reuses the same set minus --name.
type CatalogWriteConfig struct {
	name             string
	chart            string
	chartVersion     string
	appVersion       string
	helmRepoName     string
	helmRepoURL      string
	helmRepoSecret   string
	valuesFile       string
	description      string
	shortDescription string
	serviceIcon      string
	secretTypes      []string
}

// catalogHelmRepoFlagsChanged reports whether any of the --helm-repo-* flags
// were set on the command line. The repo block is replaced as a unit, so
// callers pass them together.
func catalogHelmRepoFlagsChanged(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("helm-repo-name") ||
		cmd.Flags().Changed("helm-repo-url") ||
		cmd.Flags().Changed("helm-repo-secret")
}

// readCatalogValuesFile reads the YAML file at path and returns its contents as
// a string, to be sent as the values field.
func readCatalogValuesFile(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", errors.Wrap(err, "reading values file")
	}
	return string(contents), nil
}

// catalogWriteFlags registers the flags common to create and update (i.e. every
// write flag except --name, which only create carries).
func catalogWriteFlags(cmd *cobra.Command, cfg *CatalogWriteConfig) {
	cmd.Flags().StringVar(&cfg.chart, "chart", "", "Helm chart")
	cmd.Flags().StringVar(&cfg.chartVersion, "chart-version", "", "Helm chart version")
	cmd.Flags().StringVar(&cfg.appVersion, "app-version", "", "application version")
	cmd.Flags().StringVar(&cfg.helmRepoName, "helm-repo-name", "", "Helm repository name")
	cmd.Flags().StringVar(&cfg.helmRepoURL, "helm-repo-url", "", "Helm repository URL")
	cmd.Flags().StringVar(&cfg.helmRepoSecret, "helm-repo-secret", "", "Helm repository credentials secret")
	cmd.Flags().StringVar(&cfg.valuesFile, "values-file", "", "path to a YAML file whose contents are sent as the values string field")
	cmd.Flags().StringVar(&cfg.description, "description", "", "long description")
	cmd.Flags().StringVar(&cfg.shortDescription, "short-description", "", "short description")
	cmd.Flags().StringVar(&cfg.serviceIcon, "service-icon", "", "service icon")
	cmd.Flags().StringSliceVar(&cfg.secretTypes, "secret-types", []string{}, "comma-separated secret types")
}

// NewServiceCatalogCreateCmd returns a new `epinio service catalog create` command
func NewServiceCatalogCreateCmd(client ServicesService) *cobra.Command {
	cfg := CatalogWriteConfig{}
	cmd := &cobra.Command{
		Use:   "create --name NAME --chart CHART [flags]",
		Short: "Create an Epinio catalog service",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if cfg.name == "" {
				return errors.New("--name is required")
			}

			request := models.CatalogServiceCreateRequest{
				Name:             cfg.name,
				HelmChart:        cfg.chart,
				ChartVersion:     cfg.chartVersion,
				AppVersion:       cfg.appVersion,
				Description:      cfg.description,
				ShortDescription: cfg.shortDescription,
				ServiceIcon:      cfg.serviceIcon,
				SecretTypes:      cfg.secretTypes,
				HelmRepo: models.HelmRepoRequest{
					Name:   cfg.helmRepoName,
					URL:    cfg.helmRepoURL,
					Secret: cfg.helmRepoSecret,
				},
			}

			if cfg.valuesFile != "" {
				values, err := readCatalogValuesFile(cfg.valuesFile)
				if err != nil {
					return err
				}
				request.Values = values
			}

			err := client.ServiceCatalogCreate(request)
			return errors.Wrap(err, "error creating Epinio catalog service")
		},
	}

	cmd.Flags().StringVar(&cfg.name, "name", "", "catalog service name (required)")
	catalogWriteFlags(cmd, &cfg)

	return cmd
}

// NewServiceCatalogUpdateCmd returns a new `epinio service catalog update` command
func NewServiceCatalogUpdateCmd(client ServicesService) *cobra.Command {
	cfg := CatalogWriteConfig{}
	cmd := &cobra.Command{
		Use:               "update NAME [flags]",
		Short:             "Update an Epinio catalog service",
		Long:              `Update a catalog service. Omitted flags leave the corresponding fields unchanged. Pass all --helm-repo-* flags together to replace the repo block.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: FirstArgValidator(client.CatalogMatching),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			name := args[0]

			request := models.CatalogServiceUpdateRequest{
				HelmChart:        cfg.chart,
				ChartVersion:     cfg.chartVersion,
				AppVersion:       cfg.appVersion,
				Description:      cfg.description,
				ShortDescription: cfg.shortDescription,
				ServiceIcon:      cfg.serviceIcon,
				SecretTypes:      cfg.secretTypes,
			}

			// The repo block is replaced as a unit only when the caller
			// supplied at least one --helm-repo-* flag; otherwise it is left
			// untouched (nil pointer).
			if catalogHelmRepoFlagsChanged(cmd) {
				request.HelmRepo = &models.HelmRepoRequest{
					Name:   cfg.helmRepoName,
					URL:    cfg.helmRepoURL,
					Secret: cfg.helmRepoSecret,
				}
			}

			if cfg.valuesFile != "" {
				values, err := readCatalogValuesFile(cfg.valuesFile)
				if err != nil {
					return err
				}
				request.Values = values
			}

			err := client.ServiceCatalogUpdate(name, request)
			return errors.Wrap(err, "error updating Epinio catalog service")
		},
	}

	catalogWriteFlags(cmd, &cfg)

	return cmd
}

// NewServiceCatalogDeleteCmd returns a new `epinio service catalog delete` command
func NewServiceCatalogDeleteCmd(client ServicesService) *cobra.Command {
	return &cobra.Command{
		Use:               "delete NAME",
		Short:             "Delete an Epinio catalog service",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: FirstArgValidator(client.CatalogMatching),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			err := client.ServiceCatalogDelete(args[0])
			return errors.Wrap(err, "error deleting Epinio catalog service")
		},
	}
}

type ServiceCreateConfig struct {
	wait bool
	cv   ChartValueConfig
}

// NewServiceCreateCmd returns a new `epinio service create` command
func NewServiceCreateCmd(client ServicesService) *cobra.Command {
	cfg := ServiceCreateConfig{}
	cmd := &cobra.Command{
		Use:               "create CATALOGSERVICENAME SERVICENAME",
		Short:             "Create a service SERVICENAME of an Epinio catalog service CATALOGSERVICENAME",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: FirstArgValidator(client.CatalogMatching),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			chartValues := models.ChartValueSettings{}
			for _, assignment := range cfg.cv.assigned {
				pieces := strings.SplitN(assignment, "=", 2)
				if len(pieces) < 2 {
					return errors.New("Bad --chart-value `" + assignment + "`, expected `name=value` as value")
				}
				chartValues[pieces[0]] = pieces[1]
			}

			catalogServiceName := args[0]
			serviceName := args[1]

			err := client.ServiceCreate(catalogServiceName, serviceName, cfg.wait, chartValues)
			return errors.Wrap(err, "error creating service")
		},
	}

	cmd.Flags().BoolVar(&cfg.wait, "wait", false, "Wait for deployment to complete")

	chartValueOption(cmd, &cfg.cv)
	bindFlagCompletionFunc(cmd, "chart-value", NewServiceChartValueFunc(client))

	return cmd
}

type ServiceUpdateConfig struct {
	wait      bool
	noRestart bool
	change    ChangeConfig // See configurations.go for definition
}

func NewServiceUpdateCmd(client ServicesService) *cobra.Command {
	cfg := ServiceUpdateConfig{}
	cmd := &cobra.Command{
		Use:   "update NAME [flags]",
		Short: "Update a service",
		Long:  `Update service by name and change instructions through flags.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			assignments := map[string]string{}
			for _, assignment := range cfg.change.assigned {
				pieces := strings.Split(assignment, "=")
				if len(pieces) != 2 {
					return errors.New("Bad --set assignment `" + assignment + "`, expected `name=value` as value")
				}
				assignments[pieces[0]] = pieces[1]
			}

			err := client.ServiceUpdate(args[0], cfg.wait, cfg.change.removed, assignments, cfg.noRestart)
			if err != nil {
				return errors.Wrap(err, "error creating service")
			}

			return nil
		},
		ValidArgsFunction: NewServiceMatcherFirstFunc(client),
	}

	cmd.Flags().BoolVar(&cfg.wait, "wait", false, "Wait for deployment to complete")
	cmd.Flags().BoolVar(&cfg.noRestart, "no-restart", false, "Prevent restarting bound applications after update")
	changeOptions(cmd, &cfg.change)

	return cmd
}

// NewServiceShowCmd returns a new `epinio service show` command
func NewServiceShowCmd(client ServicesService, rootCfg *RootConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "show SERVICENAME",
		Short:             "Show details of a service SERVICENAME",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: NewServiceMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			serviceName := args[0]

			err := client.ServiceShow(serviceName)
			return errors.Wrap(err, "error showing service")
		},
	}

	cmd.Flags().VarP(rootCfg.Output, "output", "o", "sets output format [text|json]")
	bindFlag(cmd, "output")
	bindFlagCompletionFunc(cmd, "output", NewStaticFlagsCompletionFunc(rootCfg.Output.Allowed))

	return cmd
}

type ServiceDeleteConfig struct {
	unbind bool
	all    bool
}

// NewServiceDeleteCmd returns a new `epinio service delete` command
func NewServiceDeleteCmd(client ServicesService) *cobra.Command {
	cfg := ServiceDeleteConfig{}
	cmd := &cobra.Command{
		Use:               "delete SERVICENAME1 [SERVICENAME2 ...]",
		Short:             "Delete one or more services",
		ValidArgsFunction: NewServiceMatcherAnyFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			if cfg.all && len(args) > 0 {
				return errors.New("Conflict between --all and named services")
			}
			if !cfg.all && len(args) == 0 {
				return errors.New("No services specified for deletion")
			}

			err := client.ServiceDelete(args, cfg.unbind, cfg.all)
			return errors.Wrap(err, "error deleting service")
		},
	}

	cmd.Flags().BoolVar(&cfg.unbind, "unbind", false, "Unbind from applications before deleting")
	cmd.Flags().BoolVar(&cfg.all, "all", false, "delete all services")

	return cmd
}

// NewServiceBindCmd returns a new `epinio service bind` command
func NewServiceBindCmd(client ServicesService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bind SERVICENAME APPNAME [SERVICENAME...]",
		Short: "Bind one or more services to an Epinio app",
		Long: `Bind services to an application.

Usage:
  Single service (backward compatible):
    epinio service bind SERVICENAME APPNAME
  
  Multiple services (batch binding - MUCH faster):
    epinio service bind APPNAME SERVICENAME1 SERVICENAME2 [SERVICENAME3...]
    
When providing 3 or more arguments, the first is treated as APPNAME and the rest as service names.
This allows binding multiple services in a single operation with only one pod restart.`,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: NewServiceAppMatcherFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			// Maintain backward compatibility:
			// - 2 args: OLD format SERVICE APP
			// - 3+ args: NEW batch format APP SERVICE1 SERVICE2 ...
			if len(args) == 2 {
				// Backward compatible: single service bind
				serviceName := args[0]
				appName := args[1]
				err := client.ServiceBind(serviceName, appName)
				return errors.Wrap(err, "error binding service")
			}

			// New batch binding format (3+ args)
			appName := args[0]
			serviceNames := args[1:]
			err := client.ServiceBatchBind(appName, serviceNames)
			return errors.Wrap(err, "error binding services")
		},
	}

	return cmd
}

// NewServiceUnbindCmd returns a new `epinio service unbind` command
func NewServiceUnbindCmd(client ServicesService) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "unbind SERVICENAME APPNAME",
		Short:             "Unbinds a service SERVICENAME from an Epinio app APPNAME",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: NewServiceAppMatcherFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			serviceName := args[0]
			appName := args[1]

			err := client.ServiceUnbind(serviceName, appName)
			return errors.Wrap(err, "error unbinding service")
		},
	}

	return cmd
}

type ServiceListConfig struct {
	all bool
}

// NewServiceListCmd returns a new `epinio service list` command
func NewServiceListCmd(client ServicesService, rootCfg *RootConfig) *cobra.Command {
	cfg := ServiceListConfig{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all the services in the targeted namespace",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			var err error
			if cfg.all {
				err = client.ServiceListAll()
			} else {
				err = client.ServiceList()
			}

			return errors.Wrap(err, "error listing services")
		},
	}

	cmd.Flags().BoolVar(&cfg.all, "all", false, "List all services")

	cmd.Flags().VarP(rootCfg.Output, "output", "o", "sets output format [text|json]")
	bindFlag(cmd, "output")
	bindFlagCompletionFunc(cmd, "output", NewStaticFlagsCompletionFunc(rootCfg.Output.Allowed))

	return cmd
}

type ServiceForwardConfig struct {
	address []string
}

// NewServicePortForwardCmd returns a new `epinio service port-forward` command
func NewServicePortForwardCmd(client ServicesService) *cobra.Command {
	cfg := ServiceForwardConfig{}
	cmd := &cobra.Command{
		Use:               "port-forward SERVICENAME [LOCAL_PORT] [...[LOCAL_PORT_N]]",
		Short:             "forward one or more local ports to a service SERVICENAME",
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: NewServiceMatcherFirstFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			serviceName := args[0]
			ports := args[1:]

			err := client.ServicePortForward(cmd.Context(), serviceName, cfg.address, ports)
			// Note: errors.Wrap (nil, "...") == nil
			return errors.Wrap(err, "error port forwarding to service")
		},
	}

	cmd.Flags().StringSliceVar(&cfg.address, "address", []string{"localhost"},
		"Addresses to listen on (comma separated). Only accepts IP addresses or localhost as a value. When localhost is supplied, kubectl will try to bind on both 127.0.0.1 and ::1 and will fail if neither of these addresses are available to bind.")

	return cmd
}

// //////////////////////////////////////////////////////////////////////////////////
/// This is the cmd-specific instance of the `chartValueOption` found in ../options.go
/// The implementation in options.go can be removed when the application commands are
/// moved into this package.

type ChartValueConfig struct {
	assigned []string
}

// chartValueOption initializes the --chartValue/-c option for the provided command
func chartValueOption(cmd *cobra.Command, cfg *ChartValueConfig) {
	cmd.Flags().StringSliceVarP(&cfg.assigned, "chart-value", "v", []string{}, "chart customization to be used")
}
