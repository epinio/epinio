package kubernetes

import (
	"github.com/spf13/cobra"
	"github.com/suse/carrier/cli/paas/ui"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Deployment
type Deployment interface {
	Deploy(*Cluster, *ui.UI, InstallationOptions) error
	Upgrade(*Cluster, *ui.UI, InstallationOptions) error
	Delete(*Cluster, *ui.UI) error
	Describe() string
	GetVersion() string
	NeededOptions() InstallationOptions
	Restore(*Cluster, *ui.UI, string) error
	Backup(*Cluster, *ui.UI, string) error
	ID() string
}

// A list of deployment that should be installed together
type DeploymentSet struct {
	Deployments   []Deployment
	NeededOptions InstallationOptions
}

// OptionsReader is the interface to the structures and objects used
// to fill InstallationOption instances with a valid value.
//
// Note, each reader has the discretion to not modify the provided
// option instance based on its state. The option's Valid flag is, for
// example, how the defaults, cli, and interactive readers communicate
// and decide which options to handle.
type OptionsReader interface {
	Read(*InstallationOption) error
}

func NewDeploymentSet(deployments ...Deployment) *DeploymentSet {
	return &DeploymentSet{
		Deployments: deployments,
	}
}

func (ds *DeploymentSet) AsCobraFlagsFor(cmd *cobra.Command) {
	ds.GatherNeededOptions()
	ds.NeededOptions.AsCobraFlagsFor(cmd)
}

// GatherNeededOptions merges all options from all deployments.
// Also ignores Values set on shared options (to avoid Deployments setting
// values for other Deployments)
func (ds *DeploymentSet) GatherNeededOptions() {
	for _, d := range ds.Deployments {
		curatedOptions := InstallationOptions{}
		for _, opt := range d.NeededOptions() {
			newOpt := opt
			if opt.DeploymentID == "" {
				newOpt.Value = ""
			}
			curatedOptions = append(curatedOptions, newOpt)
		}

		ds.NeededOptions = ds.NeededOptions.Merge(curatedOptions)
	}
}

// PopulateNeededOptions will try to give values to the needed options
// using the given OptionsReader. If none is given, the default is the
// InteractiveOptionsReader which will ask in the terminal.
// This method only populates what is possible and leaves the rest empty.
// TODO: Implement another method to validate that all options have been set.
func (ds *DeploymentSet) PopulateNeededOptions(reader OptionsReader) error {
	var err error
	newOptions := InstallationOptions{}
	for _, opt := range ds.NeededOptions {
		err = reader.Read(&opt)
		if err != nil {
			return err
		}
		newOptions = append(newOptions, opt)
	}

	ds.NeededOptions = newOptions

	return nil
}
