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
	CmdServices.AddCommand(CmdServiceCatalog)
	CmdServices.AddCommand(CmdServiceCreate)
	CmdServices.AddCommand(CmdServiceBindCreate)
	CmdServices.AddCommand(CmdServiceShow)
	CmdServices.AddCommand(CmdServiceDelete)
	CmdServices.AddCommand(CmdServiceList)
}

var CmdServiceCatalog = &cobra.Command{
	Use:   "catalog [NAME]",
	Short: "Lists all available Epinio services, or show the details of the specified one",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		if len(args) == 0 {
			err = client.ServiceCatalog()
			return errors.Wrap(err, "error listing Epinio services")
		}

		if len(args) == 1 {
			serviceName := args[0]
			err = client.ServiceCatalogShow(serviceName)
			return errors.Wrap(err, fmt.Sprintf("error showing %s Epinio service", serviceName))
		}

		return nil
	},
}

var CmdServiceCreate = &cobra.Command{
	Use:   "create CATALOGSERVICENAME SERVICENAME",
	Short: "Create an instance SERVICENAME of an Epinio service CATALOGSERVICENAME",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		catalogServiceName := args[0]
		serviceName := args[1]

		err = client.ServiceCreate(catalogServiceName, serviceName)
		return errors.Wrap(err, "error creating Epinio Service")
	},
}

var CmdServiceShow = &cobra.Command{
	Use:   "show SERVICENAME",
	Short: "Show details of a service instance SERVICENAME",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		serviceName := args[0]

		err = client.ServiceShow(serviceName)
		return errors.Wrap(err, "error showing Service")
	},
}

var CmdServiceDelete = &cobra.Command{
	Use:   "delete SERVICENAME",
	Short: "Delete service instance SERVICENAME",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		serviceName := args[0]

		err = client.ServiceDelete(serviceName)
		return errors.Wrap(err, "error deleting Service")
	},
}
var CmdServiceBindCreate = &cobra.Command{
	Use:   "bind SERVICENAME APPNAME",
	Short: "Bind a service instance SERVICENAME to an Epinio app APPNAME",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		serviceName := args[0]
		appName := args[1]

		err = client.ServiceBind(serviceName, appName)
		return errors.Wrap(err, "error binding Epinio Service")
	},
}

var CmdServiceList = &cobra.Command{
	Use:   "list",
	Short: "List all the services in the targeted namespace",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.ServiceList()
		return errors.Wrap(err, "error listing Epinio Service")
	},
}
