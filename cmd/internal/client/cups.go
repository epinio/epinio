package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/paas"
)

var ()

// CmdCreateCustomService implements the carrier create-custom-service command
var CmdCreateCustomService = &cobra.Command{
	Use:   "create-custom-service NAME (KEY VALUE)...",
	Short: "Create a custom service",
	Long:  `Create custom service by name and key/value dictionary.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 3 {
			return errors.New("Not enough arguments, expected name, key, and value")
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

		err = client.CreateCustomService(args[0], args[1:])
		if err != nil {
			return errors.Wrap(err, "error creating service")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
