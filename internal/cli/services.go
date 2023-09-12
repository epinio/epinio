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

	clicmd "github.com/epinio/epinio/internal/cli/cmd"
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
	CmdServices.AddCommand(CmdServiceUpdate)
	CmdServices.AddCommand(CmdServiceBind)
	CmdServices.AddCommand(CmdServiceUnbind)
	CmdServices.AddCommand(CmdServiceShow)
	CmdServices.AddCommand(CmdServiceDelete)
	CmdServices.AddCommand(CmdServiceList)
	CmdServices.AddCommand(CmdServicePortForward)

	chartValueOption(CmdServiceCreate)

	clicmd.BindFlagCompletionFunc(CmdServiceCreate, "chart-value",
		matchingServiceChartValueFinder)

	CmdServiceUpdate.Flags().Bool("wait", false, "Wait for deployment to complete")
	changeOptions(CmdServiceUpdate)
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

// CmdServiceUpdate implements the command: epinio service update
var CmdServiceUpdate = &cobra.Command{
	Use:   "update NAME [flags]",
	Short: "Update a service",
	Long:  `Update service by name and change instructions through flags.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		wait, err := cmd.Flags().GetBool("wait")
		if err != nil {
			return errors.Wrap(err, "error reading option --wait")
		}

		// Process the --unset and --set options into operations (removals, assignments)

		removedKeys, err := changeGetUnset(cmd)
		if err != nil {
			return err
		}

		kvAssignments, err := cmd.Flags().GetStringSlice("set")
		if err != nil {
			return errors.Wrap(err, "failed to read option --set")
		}

		assignments := map[string]string{}
		for _, assignment := range kvAssignments {
			pieces := strings.Split(assignment, "=")
			if len(pieces) != 2 {
				return errors.New("Bad --set assignment `" + assignment + "`, expected `name=value` as value")
			}
			assignments[pieces[0]] = pieces[1]
		}

		err = client.ServiceUpdate(args[0], wait, removedKeys, assignments)
		if err != nil {
			return errors.Wrap(err, "error creating service")
		}

		return nil
	},
	ValidArgsFunction: matchingServiceFinder,
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

		matches := clicmd.FilteredMatchingFinder(args, toComplete, client.ServiceMatching)
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

// /////////////////////////////////////////////////////////////////////
/// The code below is a duplicate of the code in cmd/configurations.go
/// It is needed becauses configurations has moved into package `cmd`, while services has not yet.
/// REMOVE this code when services moves into package `cmd`.

// changeOptions initializes the --unset/-u and --set/-s options for the provided command.
// It also initializes the old --remove/-r options, and marks them as deprecated.
func changeOptions(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("set", "s", []string{}, "configuration key/value assignments to add/modify")
	cmd.Flags().StringSliceP("unset", "u", []string{}, "configuration keys to remove")
	cmd.Flags().StringSliceP("remove", "r", []string{}, "(deprecated) configuration keys to remove")
	checkErr(cmd.Flags().MarkDeprecated("remove", "please use --unset instead"))

	// Note: No completion functionality. This would require asking the configuration for
	// its details so that the keys to remove can be matched. And add/modify cannot
	// check anyway.
}

func changeGetUnset(cmd *cobra.Command) ([]string, error) {
	removedKeys, err := cmd.Flags().GetStringSlice("remove")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read deprecated option --remove")
	}
	unsetKeys, err := cmd.Flags().GetStringSlice("unset")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read option --unset")
	}
	return append(unsetKeys, removedKeys...), nil
}
