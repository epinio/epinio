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

var installer = kubernetes.Installer{
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

	installer.GatherNeededOptions()
	installer.NeededOptions.AsCobraFlagsFor(CmdInstall)
}

// Install command installs carrier on a configured cluster
func Install(cmd *cobra.Command, args []string) error {
	carrier_install, cleanup_install, err := paas.BuildInstallApp(cmd.Flags(), nil)
	defer func() {
		if cleanup_install != nil {
			cleanup_install()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = carrier_install.Install(cmd, &installer)
	if err != nil {
		return errors.Wrap(err, "error installing Carrier")
	}

	// Installation complete. Run `create-org`

	carrier, cleanup, err := paas.BuildApp(cmd.Flags(), nil)
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = carrier.CreateOrg("workspace")
	if err != nil {
		return errors.Wrap(err, "error creating org")
	}

	return nil
}
