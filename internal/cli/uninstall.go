package cli

import (
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var CmdUninstall = &cobra.Command{
	Use:   "uninstall",
	Short: "uninstall Epinio from your configured kubernetes cluster",
	Long:  `uninstall Epinio PaaS from your configured kubernetes cluster`,
	Args:  cobra.ExactArgs(0),
	RunE:  Uninstall,
}

// Uninstall implement the command: epinio uninstall
// It removes epinio from a configured cluster
func Uninstall(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	ExitfIfError(checkDependencies(), "Cannot operate")

	installClient, _, err := clients.NewInstallClient(cmd.Context(), nil)
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = installClient.Uninstall(cmd.Context())
	if err != nil {
		return errors.Wrap(err, "failed to remove")
	}

	return nil
}
