package cli

import (
	"github.com/spf13/cobra"
	"github.com/suse/carrier/internal/web"
)

// CmdGui implements the carrier gui command
var CmdGui = &cobra.Command{
	Use:   "gui",
	Short: "starts the Graphical User Interface of Carrier",
	RunE: func(cmd *cobra.Command, args []string) error {
		return web.StartGui(0)
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
