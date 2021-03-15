package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/paas"
)

var ()

// CmdServicePlans implements the carrier service-plans command
var CmdServicePlans = &cobra.Command{
	Use:   "service-plans CLASS",
	Short: "Lists all plans provided by the named service class",
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

		err = client.ServicePlans(args[0])
		if err != nil {
			return errors.Wrap(err, "error listing plan")
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

		matches := app.ServiceClassMatching(toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}
