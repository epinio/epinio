package kubernetes

import (
	"errors"
	"fmt"
)

type Installer struct {
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Deployment

type Deployment interface {
	Deploy(Cluster) error
	Upgrade(Cluster) error
	SetDomain(d string)
	GetDomain() string
	Delete(Cluster) error
	Describe() string
	GetVersion() string
	CollectInput() []UserInput

	Restore(Cluster, string) error
	Backup(Cluster, string) error
}

// UserInput is a struct that holds one value required by the user.
// Various components should return an array of UserInput with all the
// configuration options they need from the user. The installer will
// ask the user all the required input before the installation happens.
type UserInput struct {
	Name        string
	Description string
	Deployment  string
}

func NewInstaller() *Installer {
	return &Installer{}
}

func (i *Installer) Install(d Deployment, cluster Cluster) error {
	fmt.Println(d.Describe())

	// Automatically set a deployment domain based on platform reported ExternalIPs
	if d.GetDomain() == "" {
		ips := cluster.GetPlatform().ExternalIPs()
		if len(ips) == 0 {
			return errors.New("Could not detect cluster ExternalIPs and no deployment domain was specified")
		}
		d.SetDomain(fmt.Sprintf("%s.nip.io", ips[0]))
	}
	return d.Deploy(cluster)
}

func (i *Installer) Delete(d Deployment, cluster Cluster) error {
	return d.Delete(cluster)
}

func (i *Installer) Upgrade(d Deployment, cluster Cluster) error {
	return d.Upgrade(cluster)
}

func (i *Installer) Backup(d Deployment, cluster Cluster, output string) error {
	return d.Backup(cluster, output)
}

func (i *Installer) Restore(d Deployment, cluster Cluster, output string) error {
	return d.Restore(cluster, output)
}
