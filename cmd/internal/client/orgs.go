package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/paas"
)

var ()

// CmdOrgs implements the carrier orgs command
var CmdOrgs = &cobra.Command{
	Use:   "orgs",
	Short: "Lists all organizations",
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

		err = client.Orgs()
		if err != nil {
			return errors.Wrap(err, "error listing orgs")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
