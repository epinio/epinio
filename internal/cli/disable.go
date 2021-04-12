package cli

import (
	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/kubernetes"
	"github.com/epinio/epinio/termui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var CmdDisable = &cobra.Command{
	Use:           "disable",
	Short:         "disable Epinio features",
	Long:          `disable Epinio features which where enabled with "epinio enable"`,
	Args:          cobra.ExactArgs(0),
	SilenceErrors: true,
	SilenceUsage:  true,
}

// TODO: Implement a flag to also delete provisioned services [TBD]
var CmdDisableInCluster = &cobra.Command{
	Use:           "services-incluster",
	Short:         "disable in-cluster services in Epinio",
	Long:          `disable in-cluster services in Epinio which will disable provisioning services on the same cluster as Epinio. Doesn't delete already provisioned services by default.`,
	Args:          cobra.ExactArgs(0),
	RunE:          DisableInCluster,
	SilenceErrors: true,
	SilenceUsage:  true,
}

var CmdDisableGoogle = &cobra.Command{
	Use:           "services-google",
	Short:         "disable Google Cloud service in Epinio",
	Long:          `disable Google Cloud services in Epinio which will disable the provisioning of those services. Doesn't delete already provisioned services by default.`,
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
		cmd, &deployments.Minibroker{Timeout: duration.ToDeployment()},
		"in-cluster services functionality has been disabled")
}

func DisableGoogle(cmd *cobra.Command, args []string) error {
	return UninstallDeployment(
		cmd, &deployments.GoogleServices{Timeout: duration.ToDeployment()},
		"Google Cloud services functionality has been disabled")
}

func UninstallDeployment(cmd *cobra.Command, deployment kubernetes.Deployment, successMessage string) error {
	uiUI := termui.NewUI()
	installClient, installCleanup, err := clients.NewInstallClient(cmd.Flags(), &kubernetes.InstallationOptions{})
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
