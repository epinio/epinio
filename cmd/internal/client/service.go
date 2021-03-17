package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/paas"
)

var ()

// CmdService implements the carrier -service command
var CmdService = &cobra.Command{
	Use:   "service NAME",
	Short: "Service information",
	Long:  `Show detailed information of the named service.`,
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

		err = client.ServiceDetails(args[0])
		if err != nil {
			return errors.Wrap(err, "error retrieving service")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, cleanup, err := paas.NewCarrierClient(cmd.Flags())
		defer func() {
			if cleanup != nil {
				cleanup()
			}
		}()

		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.ServiceMatching(toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}
