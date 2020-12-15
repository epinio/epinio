package kubernetes

import (
	"fmt"
)

type DeploymentID string

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Deployment
type Deployment interface {
	Deploy(Cluster) error
	Upgrade(Cluster) error
	SetDomain(d string)
	GetDomain() string
	Delete(Cluster) error
	Describe() string
	GetVersion() string
	NeededUserInput() InstallationOptions
	Restore(Cluster, string) error
	Backup(Cluster, string) error
	ID() DeploymentID
}

const (
	BooleanType = iota
	StringType
	IntType
)

type InstallationOptionType int

type InstallationOption struct {
	Name         string
	Value        interface{}
	Description  string
	Type         InstallationOptionType
	DeploymentID "" // If set, this option will be passed only to this deployment (private)
}

type InstallationOptions []InstallationOption

func (opts InstallationOptions) ToOptMap() map[string]InstallationOption {
	result := map[string]InstallationOption{}
	for _, opt := range opts {
		result[opt.ToOptMapKey()] = opt
	}

	return result
}

func (opt InstallationOption) ToOptMapKey() string {
	return fmt.Sprintf("%s-%s", opt.Name, opt.DeploymentID)
}

// Merge returns a merge of the two options respecting uniqueness of name+deploymentID
func (opts InstallationOptions) Merge(toMerge InstallationOptions) InstallationOptions {
	result := InstallationOptions{}
	optMap := opts.ToOptMap()
	for _, mergeOpt := range toMerge {
		optMap[mergeOpt.ToOptMapKey()] = mergeOpt
	}

	for _, v := range optMap {
		result = append(result, v)
	}

	return result
}

func (opts InstallationOptions) GetString(optionName string, deploymentID string) string {
	for _, option := range opts {
		if option.Name == optionName && string(option.DeploymentID) == deploymentID {
			result, ok := option.Value.(string)
			if !ok {
				panic("wrong type assertion")
			} else {
				return result
			}
		}
	}

	return ""
}

// A list of deployment that should be installed together
type Installer []Deployment

func (i Installer) GatherNeededOptions() InstallationOptions {
	return InstallationOptions{}
}

func (i Installer) Install(cluster Cluster, options InstallationOptions) error {
	// fmt.Println(d.Describe())

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

func (i Installer) Delete(cluster Cluster) error {
	//return d.Delete(cluster)
	return nil
}

func (i Installer) Upgrade(cluster Cluster) error {
	//return d.Upgrade(cluster)
	return nil
}

func (i Installer) Backup(cluster Cluster, output string) error {
	//return d.Backup(cluster, output)
	return nil
}

func (i Installer) Restore(cluster Cluster, output string) error {
	//return d.Restore(cluster, output)
	return nil
}
