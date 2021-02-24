package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/deployments"
	"github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/paas"
	"github.com/suse/carrier/cli/paas/ui"
)

var CmdEnable = &cobra.Command{
	Use:           "enable",
	Short:         "enable Carrier features",
	Long:          `enable Carrier features that are not enabled by default`,
	Args:          cobra.ExactArgs(0),
	SilenceErrors: true,
	SilenceUsage:  true,
}

var CmdEnableInCluster = &cobra.Command{
	Use:           "services-incluster",
	Short:         "enable in-cluster services in Carrier",
	Long:          `enable in-cluster services in Carrier which allows provisioning services which run on the same cluster as Carrier. Should be used mostly for development.`,
	Args:          cobra.ExactArgs(0),
	RunE:          EnableInCluster,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	CmdEnable.AddCommand(CmdEnableInCluster)
}

func EnableInCluster(cmd *cobra.Command, args []string) error {
	uiUI := ui.NewUI()
	installClient, installCleanup, err := paas.NewInstallClient(cmd.Flags(), &kubernetes.InstallationOptions{})
	defer func() {
		if installCleanup != nil {
			installCleanup()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}
	uiUI.Note().Msg("Minibroker installing...")
	if err := installClient.InstallDeployment(&deployments.Minibroker{Timeout: paas.DefaultTimeoutSec}, installClient.Log); err != nil {
		return err
	}
	uiUI.Note().Msg("You can now use in-cluster services")

	return nil
}
