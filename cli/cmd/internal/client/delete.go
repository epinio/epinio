package client

import (
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/paas"
)

var ()

// CmdDeleteApp implements the carrier delete command
var CmdDeleteApp = &cobra.Command{
	Use:   "delete [args]",
	Short: "Deletes an app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, cleanup, err := paas.BuildApp(cmd.Flags(), nil)
		defer func() {
			if cleanup != nil {
				cleanup()
			}
		}()

		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = app.Delete(args[0])
		if err != nil {
			return errors.Wrap(err, "error deleting app")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	pf := CmdDeleteApp.PersistentFlags()

	argToEnv := map[string]string{}

	cmd.KubeConfigFlags(pf, argToEnv)
	cmd.AddEnvToUsage(CmdDeleteApp, argToEnv)
}
