package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/paas"
)

var ()

// CmdUnbindService implements the carrier unbind-service command
var CmdUnbindService = &cobra.Command{
	Use:   "unbind-service NAME APP",
	Short: "Unbind service from an application",
	Long:  `Unbind service by name, from named application.`,
	Args:  cobra.ExactArgs(2),
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

		err = client.UnbindService(args[0], args[1])
		if err != nil {
			return errors.Wrap(err, "error creating service")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 1 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if len(args) == 1 {
			// #args == 1: app name.
			app, cleanup, _ := paas.NewCarrierClient(cmd.Flags())
			defer func() {
				if cleanup != nil {
					cleanup()
				}
			}()

			matches := app.AppsMatching(toComplete)
			return matches, cobra.ShellCompDirectiveNoFileComp
		}

		// #args == 0: service name.

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
