package cli

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/internal/cli/clients"
)

var ()

// CmdCreateOrg implements the carrier orgs command
var CmdCreateOrg = &cobra.Command{
	Use:   "create-org NAME",
	Short: "Creates an organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clients.NewCarrierClient(cmd.Flags())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.CreateOrg(args[0])
		if err != nil {
			return errors.Wrap(err, "error creating org")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
