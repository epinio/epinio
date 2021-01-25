package client

import (
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/paas"
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

func init() {
	pf := CmdUninstall.PersistentFlags()

	argToEnv := map[string]string{}

	cmd.KubeConfigFlags(pf, argToEnv)
	cmd.AddEnvToUsage(CmdUninstall, argToEnv)

	CmdUninstall.Flags().BoolP("verbose", "v", true, "Wether to print logs to stdout")
	CmdUninstall.Flags().BoolP("non-interactive", "n", false, "Whether to ask the user or not")

	carrierDeploymentSet.AsCobraFlagsFor(CmdUninstall)
}

// Uninstall command removes carrier from a configured cluster
func Uninstall(cmd *cobra.Command, args []string) error {
	installClient, _, err := paas.NewInstallClient(cmd.Flags(), nil)
	if err != nil {
		return errors.Wrap(err, "error initializing cli")
	}

	err = installClient.Uninstall(cmd, &carrierDeploymentSet)
	if err != nil {
		return errors.Wrap(err, "error uninstalling Carrier")
	}

	return nil
}
