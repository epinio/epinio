package kubernetes

import (
	"context"

	"github.com/epinio/epinio/helpers/termui"
)

type Deployment interface {
	PreDeployCheck(context.Context, *Cluster, *termui.UI, InstallationOptions) error
	PostDeleteCheck(context.Context, *Cluster, *termui.UI) error
	Deploy(context.Context, *Cluster, *termui.UI, InstallationOptions) error
	Upgrade(context.Context, *Cluster, *termui.UI, InstallationOptions) error
	Delete(context.Context, *Cluster, *termui.UI) error
	Describe() string
	GetVersion() string
	ID() string
}
