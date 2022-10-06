package cli

import (
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var ()

// CmdAppDelete implements the command: epinio app delete
var CmdAppDelete = &cobra.Command{
	Use:               "delete NAME1 [NAME2 ...]",
	Short:             "Deletes one or more applications",
	Args:              cobra.MinimumNArgs(1),
	ValidArgsFunction: matchingAppsFinder,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		client, err := usercmd.New()
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.Delete(cmd.Context(), args)
		if err != nil {
			return errors.Wrap(err, "error deleting app")
		}

		return nil
	},
}
