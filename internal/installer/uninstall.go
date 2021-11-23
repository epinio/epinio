package installer

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/termui"
	"github.com/go-logr/logr"
)

type Uninstall struct {
	cluster *kubernetes.Cluster
	log     logr.Logger
}

var _ Action = &Uninstall{}

func NewUninstall(ui *termui.UI, cluster *kubernetes.Cluster, log logr.Logger) *Uninstall {
	return &Uninstall{
		cluster: cluster,
		log:     log,
	}
}

func (u Uninstall) Apply(ctx context.Context, c Component) error {
	log := u.log.WithValues("component", c.ID, "type", c.Type)
	log.Info("apply uninstall")

	switch c.Type {
	case Helm:
		{
			if err := helmUninstall(log.V(1).WithName("helm"), c); err != nil {
				return err
			}
		}

	case YAML:
		{
			if err := yamlDelete(log.V(1).WithName("yaml"), c); err != nil {
				return err
			}
		}

	case Namespace:
		{
			if err := u.cluster.DeleteNamespace(ctx, c.Namespace); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			return nil
		}
	}

	return nil
}
