package kubernetes

import "os"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Deployment
type Deployment interface {
	Deploy(Cluster, InstallationOptions) error
	Upgrade(Cluster, InstallationOptions) error
	Delete(Cluster) error
	Describe() string
	GetVersion() string
	NeededOptions() InstallationOptions
	Restore(Cluster, string) error
	Backup(Cluster, string) error
	ID() string
}

// A list of deployment that should be installed together
type Installer struct {
	Deployments   []Deployment
	NeededOptions InstallationOptions
}

func NewInstaller(deployments ...Deployment) *Installer {
	return &Installer{
		Deployments: deployments,
	}
}

// GatherNeededOptions merges all options from all deployments.
// Also ignores Values set on shared options (to avoid Deployments setting
// values for other Deployments)
func (i *Installer) GatherNeededOptions() {
	for _, d := range i.Deployments {
		curatedOptions := InstallationOptions{}
		for _, opt := range d.NeededOptions() {
			newOpt := opt
			if opt.DeploymentID == "" {
				newOpt.Value = ""
			}
			curatedOptions = append(curatedOptions, newOpt)
		}

		i.NeededOptions = i.NeededOptions.Merge(curatedOptions)
	}
}

type OptionsReader interface {
	Read(InstallationOption) (interface{}, error)
}

// PopulateNeededOptions will try to give values to the needed options
// using the given OptionsReader. If none is given, the default is the
// InteractiveOptionsReader which will ask in the terminal.
// This method only populates what is possible and leaves the rest empty.
// TODO: Implement another method to validate that all options have been set.
func (i *Installer) PopulateNeededOptions(reader OptionsReader) error {
	if reader == nil {
		reader = NewInteractiveOptionsReader(os.Stdout, os.Stdin)
	}

	var err error
	newOptions := InstallationOptions{}
	for _, opt := range i.NeededOptions {
		opt.Value, err = reader.Read(opt)
		if err != nil {
			return err
		}
		newOptions = append(newOptions, opt)
	}

	i.NeededOptions = newOptions

	return nil
}

func (i *Installer) Install(cluster *Cluster) error {
	for _, deployment := range i.Deployments {
		options := i.NeededOptions.ForDeployment(deployment.ID())
		deployment.Deploy(*cluster, options)
	}
	// fmt.Println(d.Describe())
	//	for _, := range i {

	// // Automatically set a deployment domain based on platform reported ExternalIPs
	// if d.GetDomain() == "" {
	// 	ips := cluster.GetPlatform().ExternalIPs()
	// 	if len(ips) == 0 {
	// 		return errors.New("Could not detect cluster ExternalIPs and no deployment domain was specified")
	// 	}
	// 	d.SetDomain(fmt.Sprintf("%s.nip.io", ips[0]))
	// }
	// return d.Deploy(cluster)
	return nil
}

func (i *Installer) Delete(cluster Cluster) error {
	//return d.Delete(cluster)
	return nil
}

func (i *Installer) Upgrade(cluster Cluster) error {
	//return d.Upgrade(cluster)
	return nil
}

func (i *Installer) Backup(cluster Cluster, output string) error {
	//return d.Backup(cluster, output)
	return nil
}

func (i *Installer) Restore(cluster Cluster, output string) error {
	//return d.Restore(cluster, output)
	return nil
}
