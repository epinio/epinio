package cli

import (
	"fmt"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// CmdSettings implements the command: epinio settings
var CmdServices = &cobra.Command{
	Hidden:        true, // TODO remove me when ready
	Use:           "service",
	Short:         "Epinio service management",
	Long:          `Manage the epinio services`,
	SilenceErrors: false,
	Args:          cobra.ExactArgs(1),
}

func init() {
	CmdServices.AddCommand(CmdServiceCatalog)
	CmdServices.AddCommand(CmdServiceCreate)
	CmdServices.AddCommand(CmdServiceReleaseBindCreate)
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
	Use:   "create SERVICENAME RELEASENAME",
	Short: "Create an instance RELEASENAME of an Epinio service SERVICENAME",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		serviceName := args[0]
		serviceReleaseName := args[1]

		err = client.ServiceCreate(serviceName, serviceReleaseName)
		return errors.Wrap(err, "error creating Epinio Service")
	},
}

var CmdServiceReleaseBindCreate = &cobra.Command{
	Use:   "bind RELEASENAME APPNAME",
	Short: "Bind a service release RELEASENAME to an Epinio app APPNAME",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		serviceReleaseName := args[0]
		appName := args[1]

		err = client.ServiceReleaseBind(serviceReleaseName, appName)
		return errors.Wrap(err, "error creating Epinio Service")
	},
}
