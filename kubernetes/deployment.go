package kubernetes

import (
	"github.com/suse/carrier/paas/ui"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Deployment
type Deployment interface {
	Deploy(*Cluster, *ui.UI, InstallationOptions) error
	Upgrade(*Cluster, *ui.UI, InstallationOptions) error
	Delete(*Cluster, *ui.UI) error
	Describe() string
	GetVersion() string
	Restore(*Cluster, *ui.UI, string) error
	Backup(*Cluster, *ui.UI, string) error
	ID() string
}
