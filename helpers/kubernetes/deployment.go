package kubernetes

import (
	"context"

	"github.com/epinio/epinio/helpers/termui"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . Deployment
type Deployment interface {
	Deploy(context.Context, *Cluster, *termui.UI, InstallationOptions) error
	Upgrade(context.Context, *Cluster, *termui.UI, InstallationOptions) error
	Delete(context.Context, *Cluster, *termui.UI) error
	Describe() string
	GetVersion() string
	Restore(context.Context, *Cluster, *termui.UI, string) error
	Backup(context.Context, *Cluster, *termui.UI, string) error
	ID() string
}
