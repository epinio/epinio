package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/paas"
)

var CmdUninstall = &cobra.Command{
	Use:           "uninstall",
	Short:         "uninstall Carrier from your configured kubernetes cluster",
	Long:          `uninstall Carrier PaaS from your configured kubernetes cluster`,
	Args:          cobra.ExactArgs(0),
	RunE:          Uninstall,
	SilenceErrors: true,
	SilenceUsage:  true,
}

// Uninstall command removes carrier from a configured cluster
func Uninstall(cmd *cobra.Command, args []string) error {
	installClient, _, err := paas.NewInstallClient(cmd.Flags(), nil)
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = installClient.Uninstall(cmd)
	if err != nil {
		return err
	}

	return nil
}
