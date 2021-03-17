package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/paas"
)

var ()

// CmdServiceClasses implements the carrier service-classes command
var CmdServiceClasses = &cobra.Command{
	Use:   "service-classes",
	Short: "Lists the available service classes",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cleanup, err := paas.NewCarrierClient(cmd.Flags())
		defer func() {
			if cleanup != nil {
				cleanup()
			}
		}()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.ServiceClasses()
		if err != nil {
			return errors.Wrap(err, "error listing service classes")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
