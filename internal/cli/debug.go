package cli

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var ()

func init() {
	CmdDebug.AddCommand(CmdDebugTTY)
}

// CmdDebug implements the epinio debug command
var CmdDebug = &cobra.Command{
	Hidden:        true,
	Use:           "debug",
	Short:         "Dev Tools",
	Long:          `Developer Tools. Hidden From Regular User.`,
	Args:          cobra.ExactArgs(0),
	SilenceErrors: true,
	SilenceUsage:  true,
}

// CmdDebug implements the epinio debug command
var CmdDebugTTY = &cobra.Command{
	Use:   "tty",
	Short: "Running In a Terminal?",
	Long:  `Running In a Terminal?`,
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		if isatty.IsTerminal(os.Stdout.Fd()) {
			fmt.Println("Is Terminal")
		} else if isatty.IsCygwinTerminal(os.Stdout.Fd()) {
			fmt.Println("Is Cygwin/MSYS2 Terminal")
		} else {
			fmt.Println("Is Not Terminal")
		}
		return nil
	},
}
