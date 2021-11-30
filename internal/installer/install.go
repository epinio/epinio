package installer

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

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
			labels := map[string]string{}
			annotations := map[string]string{}
			for _, val := range c.Values {
				switch val.Type {
				case Annotation:
					annotations[val.Name] = val.Value
				case Label:
					labels[val.Name] = val.Value
				}
			}
			if err := i.cluster.CreateNamespace(ctx, c.Namespace, labels, annotations); err != nil {
				if apierrors.IsAlreadyExists(err) {
					// TODO apply labels/annotations
					return nil
				}
				return err
			}
			return nil

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
