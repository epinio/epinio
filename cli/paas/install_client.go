package paas

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/kubernetes"
	"github.com/suse/carrier/cli/paas/config"
	"github.com/suse/carrier/cli/paas/ui"
)

// InstallClient provides functionality for talking to Kubernetes for
// installing Carrier on it.
type InstallClient struct {
	kubeClient *kubernetes.Cluster
	ui         *ui.UI
	config     *config.Config
}

// Install deploys carrier to the cluster.
func (c *InstallClient) Install(cmd *cobra.Command, installer *kubernetes.Installer) error {
	c.ui.Note().Msg("Carrier installing...")

	installer.UI = c.ui

	// Hack? Override static default for system domain with a
	// function which queries the cluster for the necessary
	// data. If that data could not be found the system will fall
	// back to cli option and/or interactive entry by the user.
	//
	// NOTE: This is function is set here and not in the gitea
	// definition because the function has to have access to the
	// cluster in question, and that information is only available
	// now, not at deployment declaration time.

	domain, err := installer.NeededOptions.GetOpt("system_domain", "")
	if err != nil {
		return errors.Wrap(err, "Couldn't install carrier")
	}
	domain.DynDefaultFunc = func(o *kubernetes.InstallationOption) error {
		ips := c.kubeClient.GetPlatform().ExternalIPs()
		if len(ips) > 0 {
			domain := fmt.Sprintf("%s.nip.io", ips[0])
			o.Value = domain
		}
		// else leave invalid, to be handled by cli option
		// reader or interactive entry
		return nil
	}

	installer.PopulateNeededOptions(kubernetes.NewCLIOptionsReader(cmd))

	nonInteractive, err := cmd.Flags().GetBool("non-interactive")
	if err != nil {
		return errors.Wrap(err, "Couldn't install carrier")
	}

	if nonInteractive {
		installer.PopulateNeededOptions(kubernetes.NewDefaultOptionsReader())
	} else {
		installer.PopulateNeededOptions(kubernetes.NewInteractiveOptionsReader(os.Stdout, os.Stdin))
	}

	installer.ShowNeededOptions()

	// TODO (post MVP): Run a validation phase which perform
	// additional checks on the values. For example range limits,
	// proper syntax of the string, etc. do it as pghase, and late
	// to report all problems at once, instead of early and
	// piecemal.

	err = installer.Install(c.kubeClient)
	if err != nil {
		return errors.Wrap(err, "Couldn't install carrier")
	}

	c.ui.Success().Msg("Carrier installed.")

	return nil
}
