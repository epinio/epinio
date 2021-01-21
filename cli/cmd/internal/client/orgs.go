package client

import (
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/paas"
)

var ()

// CmdOrgs implements the carrier orgs command
var CmdOrgs = &cobra.Command{
	Use:   "orgs",
	Short: "Lists all orgs",
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

		err = app.Orgs()
		if err != nil {
			return errors.Wrap(err, "error listing orgs")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	pf := CmdOrgs.PersistentFlags()

	argToEnv := map[string]string{}

	cmd.KubeConfigFlags(pf, argToEnv)
	cmd.AddEnvToUsage(CmdOrgs, argToEnv)
}
