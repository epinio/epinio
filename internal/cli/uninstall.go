package cli

import (
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var CmdUninstall = &cobra.Command{
	Use:           "uninstall",
	Short:         "uninstall Epinio from your configured kubernetes cluster",
	Long:          `uninstall Epinio PaaS from your configured kubernetes cluster`,
	Args:          cobra.ExactArgs(0),
	RunE:          Uninstall,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Uninstall command removes epinio from a configured cluster
func Uninstall(cmd *cobra.Command, args []string) error {
	installClient, _, err := clients.NewInstallClient(cmd.Flags(), nil)
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = installClient.Uninstall(cmd)
	if err != nil {
		return err
	}

	return nil
}
