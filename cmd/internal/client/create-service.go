package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/paas"
)

var ()

// CmdCreateService implements the carrier create-service command
var CmdCreateService = &cobra.Command{
	Use:   "create-service NAME CLASS PLAN ?(KEY VALUE)...?",
	Short: "Create a service",
	Long:  `Create service by name, class, plan, and optional key/value dictionary.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 3 {
			return errors.New("Not enough arguments, expected name, class, plan, key, and value")
		}
		if len(args)%2 == 0 {
			return errors.New("Last Key has no value")
		}
		return nil
	},
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

		err = client.CreateService(args[0], args[1], args[2], args[3:])
		if err != nil {
			return errors.Wrap(err, "error creating service")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
