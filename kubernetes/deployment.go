package kubernetes

import (
	"github.com/epinio/epinio/termui"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Deployment
type Deployment interface {
	Deploy(*Cluster, *termui.UI, InstallationOptions) error
	Upgrade(*Cluster, *termui.UI, InstallationOptions) error
	Delete(*Cluster, *termui.UI) error
	Describe() string
	GetVersion() string
	Restore(*Cluster, *termui.UI, string) error
	Backup(*Cluster, *termui.UI, string) error
	ID() string
}
