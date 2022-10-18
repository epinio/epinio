package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

// CmdTarget implements the command: epinio target
var CmdTarget = &cobra.Command{
	Use:               "target [namespace]",
	Short:             "Targets an epinio-controlled namespace.",
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: matchingNamespaceFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		namespace := ""
		if len(args) > 0 {
			namespace = args[0]
		}

		err = client.Target(namespace)
		if err != nil {
			return errors.Wrap(err, "failed to set target")
		}

		return nil
	},
}
