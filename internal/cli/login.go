package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/spf13/cobra"
)

// CmdSettings implements the command: epinio settings
var CmdLogin = &cobra.Command{
	Use:           "login",
	Short:         "Epinio login",
	Long:          `MASKJMLSs`,
	SilenceErrors: true,
	SilenceUsage:  true,
	//Args:          cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, _ := usercmd.New()
		client.Login(cmd.Context(), cmd)

		return nil
	},
}
