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
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

//counterfeiter:generate -header ../../../LICENSE_HEADER . ServicesService
type ServicesService interface {
	ServiceBind(serviceName, appName string) error
	ServiceCatalog() error
	ServiceCatalogShow(ctx context.Context, serviceName string) error
	ServiceCreate(catalogName, serviceName string, wait bool, chartValues models.ChartValueSettings) error
	ServiceDelete(serviceNames []string, unbind, all bool) error
	ServiceList() error
	ServiceListAll() error
	ServicePortForward(ctx context.Context, serviceName string, address, ports []string) error
	ServiceShow(serviceName string) error
	ServiceUnbind(serviceName, appName string) error
	ServiceUpdate(serviceName string, wait bool, removed []string, assignments map[string]string) error

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
			return fmt.Errorf(`Unknown method "%s"`, args[0])
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

// NewServiceCatalogCmd returns a new `epinio service catalog` command
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

	return cmd
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
	wait   bool
	change ChangeConfig // See configurations.go for definition
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

			err := client.ServiceUpdate(args[0], cfg.wait, cfg.change.removed, assignments)
			if err != nil {
				return errors.Wrap(err, "error creating service")
			}

			return nil
		},
		ValidArgsFunction: NewServiceMatcherFirstFunc(client),
	}

	cmd.Flags().BoolVar(&cfg.wait, "wait", false, "Wait for deployment to complete")
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
		Use:               "bind SERVICENAME APPNAME",
		Short:             "Bind a service SERVICENAME to an Epinio app APPNAME",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: NewServiceAppMatcherFunc(client),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			serviceName := args[0]
			appName := args[1]

			err := client.ServiceBind(serviceName, appName)
			return errors.Wrap(err, "error binding service")
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
