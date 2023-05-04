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

	"github.com/epinio/epinio/internal/cli/usercmd"
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

	CmdServices.AddCommand(CmdServiceCatalog)
	CmdServices.AddCommand(CmdServiceCreate)
	CmdServices.AddCommand(CmdServiceBind)
	CmdServices.AddCommand(CmdServiceUnbind)
	CmdServices.AddCommand(CmdServiceShow)
	CmdServices.AddCommand(CmdServiceDelete)
	CmdServices.AddCommand(CmdServiceList)
}

var CmdServiceCatalog = &cobra.Command{
	Use:               "catalog [NAME]",
	Short:             "Lists all available Epinio catalog services, or show the details of the specified one",
	ValidArgsFunction: matchingCatalogFinder,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		if len(args) == 0 {
			err = client.ServiceCatalog()
			return errors.Wrap(err, "error listing Epinio catalog services")
		}

		if len(args) == 1 {
			serviceName := args[0]
			err = client.ServiceCatalogShow(serviceName)
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

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		catalogServiceName := args[0]
		serviceName := args[1]

		err = client.ServiceCreate(catalogServiceName, serviceName, wait)
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

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		serviceName := args[0]

		err = client.ServiceShow(serviceName)
		return errors.Wrap(err, "error showing service")
	},
}

var CmdServiceDelete = &cobra.Command{
	Use:   "delete SERVICENAME1 [SERVICENAME2 ...]",
	Short: "Delete one or more services",
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		app, err := usercmd.New(cmd.Context())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app.API.DisableVersionWarning()

		matches := filteredMatchingFinder(args, toComplete, app.ServiceMatching)
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

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
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

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		serviceName := args[0]
		appName := args[1]

		err = client.ServiceBind(serviceName, appName)
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

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		serviceName := args[0]
		appName := args[1]

		err = client.ServiceUnbind(serviceName, appName)
		return errors.Wrap(err, "error unbinding service")
	},
}

var CmdServiceList = &cobra.Command{
	Use:   "list",
	Short: "List all the services in the targeted namespace",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

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

	app, err := usercmd.New(cmd.Context())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	app.API.DisableVersionWarning()

	if len(args) == 1 {
		// #args == 1: app name.
		matches := app.AppsMatching(toComplete)
		return matches, cobra.ShellCompDirectiveNoFileComp
	}

	// #args == 0: configuration name.

	matches := app.ServiceMatching(toComplete)
	return matches, cobra.ShellCompDirectiveNoFileComp
}
