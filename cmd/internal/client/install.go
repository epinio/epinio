package client

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/kubernetes"
	"github.com/suse/carrier/paas"
)

var NeededOptions = kubernetes.InstallationOptions{
	{
		Name:        "system_domain",
		Description: "The domain you are planning to use for Carrier. Should be pointing to the traefik public IP (Leave empty to use a omg.howdoi.website domain).",
		Type:        kubernetes.StringType,
		Default:     "",
		Value:       "",
	},
}

const (
	DefaultOrganization = "workspace"
)

var CmdInstall = &cobra.Command{
	Use:           "install",
	Short:         "install Carrier in your configured kubernetes cluster",
	Long:          `install Carrier PaaS in your configured kubernetes cluster`,
	Args:          cobra.ExactArgs(0),
	RunE:          Install,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	CmdInstall.Flags().BoolP("interactive", "i", false, "Whether to ask the user or not (default not)")

	NeededOptions.AsCobraFlagsFor(CmdInstall)
}

// Install command installs carrier on a configured cluster
func Install(cmd *cobra.Command, args []string) error {
	installClient, installCleanup, err := paas.NewInstallClient(cmd.Flags(), &NeededOptions)
	defer func() {
		if installCleanup != nil {
			installCleanup()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = installClient.Install(cmd)
	if err != nil {
		return errors.Wrap(err, "error installing Carrier")
	}

	// Installation complete. Run `create-org`

	carrier_client, carrier_cleanup, err := paas.NewCarrierClient(cmd.Flags())
	defer func() {
		if carrier_cleanup != nil {
			carrier_cleanup()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	// Post Installation Tasks:
	// - Create and target a default organization, so that the
	//   user can immediately begin to push applications.
	//
	// Dev Note: The targeting is done to ensure that a carrier
	// config left over from a previous installation will contain
	// a valid organization. Without it may contain the name of a
	// now invalid organization from said previous install. This
	// then breaks push and other commands in non-obvious ways.

	err = carrier_client.CreateOrg(DefaultOrganization)
	if err != nil {
		return errors.Wrap(err, "error creating org")
	}

	err = carrier_client.Target(DefaultOrganization)
	if err != nil {
		return errors.Wrap(err, "failed to set target")
	}

	return nil
}
