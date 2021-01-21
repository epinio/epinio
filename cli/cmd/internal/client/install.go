package client

import (
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/deployments"
	"github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/paas"
)

const (
	DefaultTimeoutSec = 300
)

var carrierDeploymentSet = kubernetes.DeploymentSet{
	Deployments: []kubernetes.Deployment{
		&deployments.Traefik{Timeout: DefaultTimeoutSec},
		&deployments.Quarks{Timeout: DefaultTimeoutSec},
		&deployments.Gitea{Timeout: DefaultTimeoutSec},
		&deployments.Eirini{Timeout: DefaultTimeoutSec},
		&deployments.Registry{Timeout: DefaultTimeoutSec},
		&deployments.Tekton{Timeout: DefaultTimeoutSec},
	},
}

var CmdInstall = &cobra.Command{
	Use:   "install",
	Short: "install Carrier in your configured kubernetes cluster",
	Long:  `install Carrier PaaS in your configured kubernetes cluster`,
	Args:  cobra.ExactArgs(0),
	RunE:  Install,
}

func init() {
	pf := CmdInstall.PersistentFlags()

	argToEnv := map[string]string{}

	cmd.KubeConfigFlags(pf, argToEnv)
	cmd.AddEnvToUsage(CmdInstall, argToEnv)

	CmdInstall.Flags().BoolP("verbose", "v", true, "Wether to print logs to stdout")
	CmdInstall.Flags().BoolP("non-interactive", "n", false, "Whether to ask the user or not")

	carrierDeploymentSet.AsCobraFlagsFor(CmdInstall)
}

// Install command installs carrier on a configured cluster
func Install(cmd *cobra.Command, args []string) error {
	install_client, install_cleanup, err := paas.NewInstallClient(cmd.Flags(), nil)
	defer func() {
		if install_cleanup != nil {
			install_cleanup()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = install_client.Install(cmd, &carrierDeploymentSet)
	if err != nil {
		return errors.Wrap(err, "error installing Carrier")
	}

	// Installation complete. Run `create-org`

	carrier_client, carrier_cleanup, err := paas.NewCarrierClient(cmd.Flags(), nil)
	defer func() {
		if carrier_cleanup != nil {
			carrier_cleanup()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = carrier_client.CreateOrg("workspace")
	if err != nil {
		return errors.Wrap(err, "error creating org")
	}

	return nil
}
