package cli

import (
	"fmt"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/epinio/epinio/internal/cli/clients"
	"github.com/epinio/epinio/internal/duration"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var CmdEnable = &cobra.Command{
	Use:           "enable",
	Short:         "enable Epinio features",
	Long:          `enable Epinio features that are not enabled by default`,
	SilenceErrors: true,
	SilenceUsage:  true,
	Args:          cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := cmd.Usage(); err != nil {
			return err
		}
		return fmt.Errorf(`Unknown method "%s"`, args[0])
	},
}

var CmdEnableInCluster = &cobra.Command{
	Use:   "services-incluster",
	Short: "enable in-cluster services in Epinio",
	Long:  `enable in-cluster services in Epinio which allows provisioning services which run on the same cluster as Epinio. Should be used mostly for development.`,
	Args:  cobra.ExactArgs(0),
	RunE:  EnableInCluster,
}

var CmdEnableGoogle = &cobra.Command{
	Use:   "services-google",
	Short: "enable Google Cloud services in Epinio",
	Long:  `enable Google Cloud services in Epinio which allows provisioning those kind of services.`,
	Args:  cobra.ExactArgs(0),
	RunE:  EnableGoogle,
}

func init() {
	CmdEnableGoogle.Flags().String("service-account-json", "", "the path to the service_account_json for Google Cloud authentication [required]")
	CmdEnableGoogle.MarkFlagRequired("service-account-json")
	CmdEnable.AddCommand(CmdEnableInCluster)
	CmdEnable.AddCommand(CmdEnableGoogle)
}

// EnableInCluster implements: epinio enable services-incluster
func EnableInCluster(cmd *cobra.Command, args []string) error {
	err := InstallDeployment(
		cmd, &deployments.Minibroker{Timeout: duration.ToDeployment()},
		kubernetes.InstallationOptions{},
		"You can now use in-cluster services")
	if err != nil {
		return err
	}

	ui := termui.NewUI()
	ui.Exclamation().Msg("Beware, minibroker requires some time to catalog and declare the available services.")
	ui.Exclamation().Msg("And ServiceCatalog some more to pick up these declarations.")
	ui.Exclamation().Msg("Please do not expect `list-classes` to show them instantly.")
	ui.Exclamation().Msg("If they are not present when you try to list them, try again a bit later.")

	return nil
}

// EnableGoogle implements: epinio enable services-google
func EnableGoogle(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	serviceAccountJSONPath, err := cmd.Flags().GetString("service-account-json")
	if err != nil {
		return errors.Wrap(err, "error reading option --service-account-json")
	}

	return InstallDeployment(
		cmd, &deployments.GoogleServices{Timeout: duration.ToDeployment()},
		kubernetes.InstallationOptions{
			{
				Name:         "service-account-json",
				Description:  "The path to the service account json file used to authenticate with Google Cloud",
				Type:         kubernetes.StringType,
				Default:      "",
				Value:        serviceAccountJSONPath,
				DeploymentID: deployments.GoogleServicesDeploymentID,
			},
		},
		"You can now use Google Cloud services")
}

// InstallDeployment is a helper which installs the single referenced deployment
func InstallDeployment(cmd *cobra.Command, deployment kubernetes.Deployment, opts kubernetes.InstallationOptions, successMessage string) error {
	cmd.SilenceUsage = true

	uiUI := termui.NewUI()
	installClient, installCleanup, err := clients.NewInstallClient(cmd.Context(), &opts)
	defer func() {
		if installCleanup != nil {
			installCleanup()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	uiUI.Note().Msg(deployment.ID() + " installing...")

	if err := installClient.InstallDeployment(cmd.Context(), deployment, installClient.Log); err != nil {
		return errors.Wrap(err, "failed to deploy")
	}

	uiUI.Note().Msg(successMessage)

	return nil
}
