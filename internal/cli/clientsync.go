package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

// CmdClientSync implements the command: epinio client-sync
var CmdClientSync = &cobra.Command{
	Use:   "client-sync",
	Short: "Downloads a client binary matching the currently logged server",
	Long:  `Synchronizes the epinio client with the server by downloading the matching binary and replacing the current one.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New(cmd.Context())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.ClientSync()
		if err != nil {
			return errors.Wrap(err, "error syncing the Epinio client")
		}

		return nil
	},
}
