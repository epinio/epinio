package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/spf13/cobra"
)

// CmdLogin implements the command: epinio login
var CmdLogin = &cobra.Command{
	Use:           "login [address]",
	Short:         "Epinio login to the server",
	Long:          `The login command will setup the settings file with the provided credentials`,
	SilenceErrors: true,
	SilenceUsage:  true,
	Args:          cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return err
		}

		return client.Login(args[0])
	},
}
