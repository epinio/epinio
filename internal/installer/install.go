package installer

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/go-logr/logr"
)

type Install struct {
	cluster *kubernetes.Cluster
	log     logr.Logger
	ca      *ComponentActions
}

var _ Action = &Install{}

func NewInstall(cluster *kubernetes.Cluster, log logr.Logger, ca *ComponentActions) *Install {
	return &Install{
		ca:      ca,
		cluster: cluster,
		log:     log,
	}
}

func (i Install) Apply(ctx context.Context, c Component) error {
	log := i.log.WithValues("component", c.ID, "type", c.Type)
	log.Info("apply install")

	for _, chk := range c.PreDeploy {
		log.V(2).Info("pre deploy", "checkType", string(chk.Type))
		if err := i.ca.Run(ctx, c, chk); err != nil {
			return err
		}
	}

	switch c.Type {
	case Helm:
		{
			if err := helmUpdate(log.V(1).WithName("helm"), c); err != nil {
				return err
			}
		}

	case YAML:
		{
			if err := yamlApply(log.V(1).WithName("yaml"), c); err != nil {
				return err
			}
		}

	case Namespace:
		{
			if err := namespaceUpsert(ctx, i.cluster, c); err != nil {
				return err
			}
		}
	}

	for _, chk := range c.WaitComplete {
		log.V(2).Info("wait complete", "checkType", string(chk.Type))

		if err := i.ca.Run(ctx, c, chk); err != nil {
			return err
		}
	}

	return nil
}
