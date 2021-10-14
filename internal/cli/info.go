// +build app

package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(CmdCompletion)
	rootCmd.AddCommand(CmdNamespace)
	rootCmd.AddCommand(CmdAppPush) // shorthand access to `app push`.
	rootCmd.AddCommand(CmdApp)
	rootCmd.AddCommand(CmdTarget)
	rootCmd.AddCommand(CmdService)
	rootCmd.AddCommand(CmdInfo)
	// Hidden command providing developer tools
	rootCmd.AddCommand(CmdDebug)
}

// CmdInfo implements the command: epinio info
var CmdInfo = &cobra.Command{
	Use:   "info",
	Short: "Shows information about the Epinio environment",
	Long:  `Shows status and versions for epinio's server-side components.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.Info()
		if err != nil {
			return errors.Wrap(err, "error retrieving Epinio environment information")
		}

		return nil
	},
}
