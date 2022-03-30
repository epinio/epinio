package cli

import (
	"fmt"

	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// CmdServices implements the command: epinio services
var CmdServices = &cobra.Command{
	Hidden:        true, // TODO remove me when ready
	Use:           "service",
	Aliases:       []string{"services"},
	Short:         "Epinio service management",
	Long:          `Manage the epinio services`,
	SilenceErrors: false,
	Args:          cobra.ExactArgs(1),
}

func init() {
	CmdServices.AddCommand(CmdServiceCatalog)
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
