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

package cli

import (
	"fmt"
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// CmdServices implements the command: epinio services
var CmdServices = &cobra.Command{
	Use:           "service",
	Aliases:       []string{"services"},
	Short:         "Epinio service management",
	Long:          `Manage the epinio services`,
	SilenceErrors: false,
	Args:          cobra.ExactArgs(1),
}

func init() {
	CmdServiceCreate.Flags().Bool("wait", false, "Wait for deployment to complete")
	CmdServiceDelete.Flags().Bool("unbind", false, "Unbind from applications before deleting")
	CmdServiceList.Flags().Bool("all", false, "List all services")
	CmdServiceDelete.Flags().Bool("all", false, "delete all services")
	CmdServicePortForward.Flags().StringSliceVar(&servicePortForwardAddress, "address", []string{"localhost"}, "Addresses to listen on (comma separated). Only accepts IP addresses or localhost as a value. When localhost is supplied, kubectl will try to bind on both 127.0.0.1 and ::1 and will fail if neither of these addresses are available to bind.")

	CmdServices.AddCommand(CmdServiceCatalog)
	CmdServices.AddCommand(CmdServiceCreate)
	CmdServices.AddCommand(CmdServiceBind)
	CmdServices.AddCommand(CmdServiceUnbind)
	CmdServices.AddCommand(CmdServiceShow)
	CmdServices.AddCommand(CmdServiceDelete)
	CmdServices.AddCommand(CmdServiceList)
	CmdServices.AddCommand(CmdServicePortForward)

	chartValueOption(CmdServiceCreate)
	err := CmdServiceCreate.RegisterFlagCompletionFunc("chart-value",
		matchingServiceChartValueFinder)
	checkErr(err)
}

var CmdServiceCatalog = &cobra.Command{
	Use:               "catalog [NAME]",
	Short:             "Lists all available Epinio catalog services, or show the details of the specified one",
	ValidArgsFunction: matchingCatalogFinder,
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

var CmdServiceCreate = &cobra.Command{
	Use:               "create CATALOGSERVICENAME SERVICENAME",
	Short:             "Create a service SERVICENAME of an Epinio catalog service CATALOGSERVICENAME",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: matchingCatalogFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		wait, err := cmd.Flags().GetBool("wait")
		if err != nil {
			return errors.Wrap(err, "error reading option --wait")
		}

		cvAssignments, err := cmd.Flags().GetStringSlice("chart-value")
		if err != nil {
			return errors.Wrap(err, "failed to read option --chart-value")
		}
		chartValues := models.ChartValueSettings{}
		for _, assignment := range cvAssignments {
			pieces := strings.SplitN(assignment, "=", 2)
			if len(pieces) < 2 {
				return errors.New("Bad --chart-value `" + assignment + "`, expected `name=value` as value")
			}
			chartValues[pieces[0]] = pieces[1]
		}

		catalogServiceName := args[0]
		serviceName := args[1]

		err = client.ServiceCreate(catalogServiceName, serviceName, wait, chartValues)
		return errors.Wrap(err, "error creating service")
	},
}

var CmdServiceShow = &cobra.Command{
	Use:               "show SERVICENAME",
	Short:             "Show details of a service SERVICENAME",
	Args:              cobra.ExactArgs(1),
	ValidArgsFunction: matchingServiceFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		serviceName := args[0]

		err := client.ServiceShow(serviceName)
		return errors.Wrap(err, "error showing service")
	},
}

var CmdServiceDelete = &cobra.Command{
	Use:   "delete SERVICENAME1 [SERVICENAME2 ...]",
	Short: "Delete one or more services",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {

		client.API.DisableVersionWarning()

		matches := filteredMatchingFinder(args, toComplete, client.ServiceMatching)
		return matches, cobra.ShellCompDirectiveNoFileComp
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		unbind, err := cmd.Flags().GetBool("unbind")
		if err != nil {
			return errors.Wrap(err, "error reading option --unbind")
		}

		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return errors.Wrap(err, "error reading option --all")
		}

		if all && len(args) > 0 {
			return errors.New("Conflict between --all and named services")
		}
		if !all && len(args) == 0 {
			return errors.New("No services specified for deletion")
		}

		err = client.ServiceDelete(args, unbind, all)
		return errors.Wrap(err, "error deleting service")
	},
}
var CmdServiceBind = &cobra.Command{
	Use:               "bind SERVICENAME APPNAME",
	Short:             "Bind a service SERVICENAME to an Epinio app APPNAME",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: findServiceApp,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		serviceName := args[0]
		appName := args[1]

		err := client.ServiceBind(serviceName, appName)
		return errors.Wrap(err, "error binding service")
	},
}

var CmdServiceUnbind = &cobra.Command{
	Use:               "unbind SERVICENAME APPNAME",
	Short:             "Unbinds a service SERVICENAME from an Epinio app APPNAME",
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: findServiceApp,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		serviceName := args[0]
		appName := args[1]

		err := client.ServiceUnbind(serviceName, appName)
		return errors.Wrap(err, "error unbinding service")
	},
}

var CmdServiceList = &cobra.Command{
	Use:   "list",
	Short: "List all the services in the targeted namespace",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		all, err := cmd.Flags().GetBool("all")
		if err != nil {
			return errors.Wrap(err, "error reading option --all")
		}

		if all {
			err = client.ServiceListAll()
		} else {
			err = client.ServiceList()
		}

		return errors.Wrap(err, "error listing services")
	},
}

func findServiceApp(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	client.API.DisableVersionWarning()

	if len(args) == 1 {
		// #args == 1: app name.
		matches := client.AppsMatching(toComplete)
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	// #args == 0: configuration name.

	matches := client.ServiceMatching(toComplete)
	return matches, cobra.ShellCompDirectiveNoFileComp
}

var (
	servicePortForwardAddress []string
)

// CmdServicePortForward implements the command: epinio service port-forward
var CmdServicePortForward = &cobra.Command{
	Use:               "port-forward SERVICENAME [LOCAL_PORT] [...[LOCAL_PORT_N]]",
	Short:             "forward one or more local ports to a service SERVICENAME",
	Args:              cobra.MinimumNArgs(2),
	ValidArgsFunction: matchingServiceFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		serviceName := args[0]
		ports := args[1:]

		err := client.ServicePortForward(cmd.Context(), serviceName, servicePortForwardAddress, ports)
		// Note: errors.Wrap (nil, "...") == nil
		return errors.Wrap(err, "error port forwarding to service")
	},
}
