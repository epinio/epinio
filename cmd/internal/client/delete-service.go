package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/paas"
)

var ()

// CmdDeleteService implements the carrier delete-service command
var CmdDeleteService = &cobra.Command{
	Use:   "delete-service NAME",
	Short: "Delete a service",
	Long:  `Delete service by name.`,
	Args:  cobra.ExactArgs(1),
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

		err = client.DeleteService(args[0])
		if err != nil {
			return errors.Wrap(err, "error creating service")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		app, cleanup, _ := paas.NewCarrierClient(cmd.Flags())
		defer func() {
			if cleanup != nil {
				cleanup()
			}
		}()

		matches := app.ServiceMatching(toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}
