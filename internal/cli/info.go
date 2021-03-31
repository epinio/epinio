package cli

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/internal/cli/clients"
)

var ()

// CmdInfo implements the carrier info command
var CmdInfo = &cobra.Command{
	Use:   "info",
	Short: "Shows information about the Carrier environment",
	Long:  `Shows status and version for Kubernetes, Gitea, Tekton, Quarks and Eirini.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clients.NewCarrierClient(cmd.Flags())
		if err != nil {
			return errors.Wrap(err, "error initializing cli")
		}

		err = client.Info()
		if err != nil {
			return errors.Wrap(err, "error retrieving Carrier environment information")
		}

		return nil
	},
	SilenceErrors: true,
	SilenceUsage:  true,
}
