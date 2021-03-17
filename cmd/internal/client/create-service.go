package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/paas"
)

var ()

func init() {
	CmdCreateService.Flags().Bool("dont-wait", false, "Return immediately, without waiting for the service to be provisioned")
}

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

		dw, err := cmd.Flags().GetBool("dont-wait")
		if err != nil {
			return err
		}
		waitforProvision := !dw

		err = client.CreateService(args[0], args[1], args[2], args[3:], waitforProvision)
		if err != nil {
			return errors.Wrap(err, "error creating service")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 2 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if len(args) == 0 {
			// #args == 0: service name. new. nothing to match
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

		if len(args) == 2 {
			// #args == 2: service plan name.
			matches := app.ServicePlanMatching(args[1], toComplete)
			return matches, cobra.ShellCompDirectiveNoFileComp
		}

		// #args == 1: service class name.
		matches := app.ServiceClassMatching(toComplete)
		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}
