package cli

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/internal/cli/clients"
)

var ()

// CmdTarget implements the carrier target command
var CmdTarget = &cobra.Command{
	Use:   "target [org]",
	Short: "Targets an organization in Carrier.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, cleanup, err := clients.NewCarrierClient(cmd.Flags())
		defer func() {
			if cleanup != nil {
				cleanup()
			}
		}()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		org := ""
		if len(args) > 0 {
			org = args[0]
		}

		err = client.Target(org)
		if err != nil {
			return errors.Wrap(err, "failed to set target")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, cleanup, err := clients.NewCarrierClient(cmd.Flags())
		defer func() {
			if cleanup != nil {
				cleanup()
			}
		}()

		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.OrgsMatching(toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}
