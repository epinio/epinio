package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// CmdApp implements the  command: epinio app
var CmdEvents = &cobra.Command{
	Use:           "events",
	Short:         "Stream events about resources in namespaces accessible to the user",
	Long:          "This command streams events about resources in any namespace the current user has access too",
	SilenceErrors: false,
	Args:          cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.EventStream()
		return errors.Wrap(err, "error streaming events")
	},
}
