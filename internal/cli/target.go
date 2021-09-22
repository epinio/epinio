package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

// CmdTarget implements the command: epinio target
var CmdTarget = &cobra.Command{
	Use:   "target [namespace]",
	Short: "Targets an epinio-controlled namespace.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
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
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		app, err := usercmd.New()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		matches := app.OrgsMatching(toComplete)

		return matches, cobra.ShellCompDirectiveNoFileComp
	},
}
