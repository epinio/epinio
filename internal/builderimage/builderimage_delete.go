package builderimage

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Delete removes the named BuilderImage CR from the cluster.
func Delete(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	name string,
) error {
	client, err := cluster.ClientBuilderImage()
	if err != nil {
		return err
	}

	return client.
		Namespace(helmchart.Namespace()).
		Delete(ctx, name, metav1.DeleteOptions{})
}
