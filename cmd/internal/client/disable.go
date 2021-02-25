package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/deployments"
	"github.com/suse/carrier/kubernetes"
	"github.com/suse/carrier/paas"
	"github.com/suse/carrier/paas/ui"
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

var CmdDisableGoogle = &cobra.Command{
	Use:           "services-google",
	Short:         "disable Google Cloud service in Carrier",
	Long:          `disable Google Cloud services in Carrier which will disable the provisioning of those services. Doesn't delete already provisioned services by default.`,
	Args:          cobra.ExactArgs(0),
	RunE:          DisableGoogle,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	CmdDisable.AddCommand(CmdDisableInCluster)
	CmdDisable.AddCommand(CmdDisableGoogle)
}

func DisableInCluster(cmd *cobra.Command, args []string) error {
	return UninstallDeployment(
		cmd, &deployments.Minibroker{Timeout: paas.DefaultTimeoutSec},
		"in-cluster services functionality has been disabled")
}

func DisableGoogle(cmd *cobra.Command, args []string) error {
	return UninstallDeployment(
		cmd, &deployments.GoogleServices{Timeout: paas.DefaultTimeoutSec},
		"Google Cloud services functionality has been disabled")
}

func UninstallDeployment(cmd *cobra.Command, deployment kubernetes.Deployment, successMessage string) error {
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
	uiUI.Note().Msg(deployment.ID() + " uninstalling...")
	if err := installClient.UninstallDeployment(deployment, installClient.Log); err != nil {
		return err
	}
	uiUI.Note().Msg(successMessage)

	return nil
}
