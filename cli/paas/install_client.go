package paas

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/deployments"
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
func (c *InstallClient) Install(cmd *cobra.Command, deployments *kubernetes.DeploymentSet) error {
	c.ui.Note().Msg("Carrier installing...")

	// Hack? Override static default for system domain with a
	// function which queries the cluster for the necessary
	// data. If that data could not be found the system will fall
	// back to cli option and/or interactive entry by the user.
	//
	// NOTE: This is function is set here and not in the gitea
	// definition because the function has to have access to the
	// cluster in question, and that information is only available
	// now, not at deployment declaration time.

	domain, err := deployments.NeededOptions.GetOpt("system_domain", "")
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

	deployments.PopulateNeededOptions(kubernetes.NewCLIOptionsReader(cmd))

	interactive, err := cmd.Flags().GetBool("interactive")
	if err != nil {
		return errors.Wrap(err, "Couldn't install carrier")
	}

	if interactive {
		deployments.PopulateNeededOptions(kubernetes.NewInteractiveOptionsReader(os.Stdout, os.Stdin))
	} else {
		deployments.PopulateNeededOptions(kubernetes.NewDefaultOptionsReader())
	}

	c.showInstallConfiguration(deployments)

	// TODO (post MVP): Run a validation phase which perform
	// additional checks on the values. For example range limits,
	// proper syntax of the string, etc. do it as pghase, and late
	// to report all problems at once, instead of early and
	// piecemal.

	err = c.deploySet(deployments)
	if err != nil {
		return errors.Wrap(err, "Couldn't install carrier")
	}

	c.ui.Success().Msg("Carrier installed.")

	return nil
}

// Uninstall removes carrier from the cluster.
func (c *InstallClient) Uninstall(cmd *cobra.Command, deploymentset *kubernetes.DeploymentSet) error {
	c.ui.Note().Msg("Carrier uninstalling...")

	eiriniDeploymentID := (&deployments.Eirini{}).ID()
	// Eirini deployment first
	for _, d := range deploymentset.Deployments {
		if d.ID() == eiriniDeploymentID {
			err := d.Delete(c.kubeClient, c.ui)
			if err != nil {
				return err
			}
		}
	}

	size := len(deploymentset.Deployments)
	for index := range deploymentset.Deployments {
		d := deploymentset.Deployments[size-index-1]
		if d.ID() != eiriniDeploymentID {
			err := d.Delete(c.kubeClient, c.ui)
			if err != nil {
				return err
			}
		}
	}

	c.ui.Success().Msg("Carrier uninstalled.")

	return nil
}

// showInstallConfiguration prints the options and their values to stdout, to
// inform the user of the detected and chosen configuration
func (c *InstallClient) showInstallConfiguration(ds *kubernetes.DeploymentSet) {
	m := c.ui.Normal()
	for _, opt := range ds.NeededOptions {
		name := "  :compass: " + opt.Name
		switch opt.Type {
		case kubernetes.BooleanType:
			m = m.WithBoolValue(name, opt.Value.(bool))
		case kubernetes.StringType:
			m = m.WithStringValue(name, opt.Value.(string))
		case kubernetes.IntType:
			m = m.WithIntValue(name, opt.Value.(int))
		}
	}
	m.Msg("Configuration...")
}

// deploySet deploys all the deployments in the set, in order.
func (c *InstallClient) deploySet(ds *kubernetes.DeploymentSet) error {
	for _, deployment := range ds.Deployments {
		options := ds.NeededOptions.ForDeployment(deployment.ID())
		err := deployment.Deploy(c.kubeClient, c.ui, options)
		if err != nil {
			return err
		}
	}
	return nil
}
