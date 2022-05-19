// Namespace contains the API handlers to manage namespaces.
package namespace

import (
	"context"
)

// Controller represents all functionality of the API related to namespaces
type Controller struct {
	namespaceService NamespaceService
}

func NewController(namespaceService NamespaceService) *Controller {
	return &Controller{namespaceService}
}

type NamespaceService interface {
	Exists(ctx context.Context, namespace string) (bool, error)
	Create(ctx context.Context, namespace string) error
}
