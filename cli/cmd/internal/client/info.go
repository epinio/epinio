package client

import (
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/paas"
)

var ()

// CmdInfo implements the carrier info command
var CmdInfo = &cobra.Command{
	Use:   "info",
	Short: "Shows information about the Carrier environment",
	Long:  `Shows status and version for Kubernetes, Gitea, Tekton, Quarks and Eirini.`,
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

		err = app.Info()
		if err != nil {
			return errors.Wrap(err, "error retrieving Carrier environment information")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	pf := CmdInfo.PersistentFlags()

	argToEnv := map[string]string{}

	cmd.KubeConfigFlags(pf, argToEnv)
	cmd.AddEnvToUsage(CmdInfo, argToEnv)
}
