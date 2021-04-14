package cli

import (
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

// CmdOrgCreate implements the epinio `orgs create` command
var CmdOrgCreate = &cobra.Command{
	Use:   "create NAME",
	Short: "Creates an organization",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clients.NewEpinioClient(cmd.Flags())
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
