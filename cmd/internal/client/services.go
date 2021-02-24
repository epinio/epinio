package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/paas"
)

var ()

// CmdServices implements the carrier services command
var CmdServices = &cobra.Command{
	Use:   "services",
	Short: "Lists all services",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cleanup, err := paas.NewCarrierClient(cmd.Flags(), nil)
		defer func() {
			if cleanup != nil {
				cleanup()
			}
		}()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.Services()
		if err != nil {
			return errors.Wrap(err, "error listing services")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
