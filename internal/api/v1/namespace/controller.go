// Namespace contains the API handlers to manage namespaces.
package namespace

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
)

// Controller represents all functionality of the API related to namespaces
type Controller struct {
	namespaceService NamespaceService
	authService      AuthService
}

func NewController(namespaceService NamespaceService, authService AuthService) *Controller {
	return &Controller{namespaceService, authService}
}

//counterfeiter:generate . NamespaceService
type NamespaceService interface {
	Exists(ctx context.Context, namespace string) (bool, error)
	Create(ctx context.Context, namespace string) error
}

//counterfeiter:generate . AuthService
type AuthService interface {
	AddNamespaceToUser(ctx context.Context, username, namespace string) error
}
