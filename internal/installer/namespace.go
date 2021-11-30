package installer

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func namespaceUpsert(ctx context.Context, cluster *kubernetes.Cluster, c Component) error {
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
	if err := cluster.CreateNamespace(ctx, c.Namespace, labels, annotations); err != nil {
		if apierrors.IsAlreadyExists(err) {
			ns, err := cluster.GetNamespace(ctx, c.Namespace)
			if err != nil {
				return err
			}

			for n, v := range labels {
				ns.Labels[n] = v
			}
			for n, v := range annotations {
				ns.Annotations[n] = v
			}
			return cluster.UpdateNamespace(ctx, c.Namespace, ns.Labels, ns.Annotations)
		}
		return err
	}
	return nil

}
