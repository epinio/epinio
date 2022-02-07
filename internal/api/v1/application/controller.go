// Application contains the API handlers to manage applications. Except for app environment.
package application

import (
	"context"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/namespaces"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
)

// Controller represents all functionality of the API related to applications
type Controller struct {
}

func (c Controller) validateNamespace(ctx context.Context, cluster *kubernetes.Cluster, namespace string) apierror.APIErrors {
	exists, err := namespaces.Exists(ctx, cluster, namespace)

	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.NamespaceIsNotKnown(namespace)
	}

	return nil
}
