package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/deployments"
	"github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/paas"
	"github.com/suse/carrier/cli/paas/ui"
)

var CmdDisable = &cobra.Command{
	Use:           "disable",
	Short:         "disable Carrier features",
	Long:          `disable Carrier features which where enabled with "carrier enable"`,
	Args:          cobra.ExactArgs(0),
	SilenceErrors: true,
	SilenceUsage:  true,
}

// TODO: Implement a flag to also delete provisioned services [TBD]
var CmdDisableInCluster = &cobra.Command{
	Use:           "services-incluster",
	Short:         "disable in-cluster services in Carrier",
	Long:          `disable in-cluster services in Carrier which will disable provisioning services on the same cluster as Carrier. Doesn't delete already provisioned services by default.`,
	Args:          cobra.ExactArgs(0),
	RunE:          DisableInCluster,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	CmdDisable.AddCommand(CmdDisableInCluster)
}

func DisableInCluster(cmd *cobra.Command, args []string) error {
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
	uiUI.Note().Msg("Minibroker uninstalling...")
	if err := installClient.UninstallDeployment(&deployments.Minibroker{Timeout: paas.DefaultTimeoutSec}, installClient.Log); err != nil {
		return err
	}
	uiUI.Note().Msg("in-cluster services functionality has been disabled")

	return nil
}
