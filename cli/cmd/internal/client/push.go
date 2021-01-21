package client

import (
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/paas"
)

var ()

// CmdPush implements the carrier orgs command
var CmdPush = &cobra.Command{
	Use:   "push [args]",
	Short: "Push an app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, cleanup, err := paas.NewCarrierClient(cmd.Flags(), nil)
		defer func() {
			if cleanup != nil {
				cleanup()
			}
		}()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		// TODO - add a parameter for path
		err = app.Push(args[0], ".")
		if err != nil {
			return errors.Wrap(err, "error pushing app")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	pf := CmdPush.PersistentFlags()

	argToEnv := map[string]string{}

	cmd.KubeConfigFlags(pf, argToEnv)
	cmd.AddEnvToUsage(CmdPush, argToEnv)
}
