// Package interfaces defines various interfaces used in Epinio whose
// definition in a more specific package would cause import loops.
package interfaces

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

// Service is the interface for access to any service, whether
// catalog-based or custom.
type Service interface {
	// Name returns the name of the service instance
	Name() string
	// Username returns the name of the service user
	User() string
	// Org returns the name of the organization the service
	// instance was created in
	Org() string
	// GetBinding returns a kube secret resource representing the
	// binding between the service instance and the specified
	// application.
	GetBinding(ctx context.Context, appName string, username string) (*corev1.Secret, error)
	// DeleteBinding removes the binding between the service
	// instance and the specified application.
	DeleteBinding(ctx context.Context, appName, org string) error
	// Delete destroys the service instance
	Delete(context.Context) error
	// Status returns a string describing the status of
	// provisioning the service instance.
	Status(context.Context) (string, error)
	// Details returns a map from strings to strings detailing the
	// service instance's configuration.
	Details(context.Context) (map[string]string, error)
	// WaitForProvision returns when the service instance is
	// completely provisioned, or after giving up on the service
	// instance to be provisioned.
	WaitForProvision(context.Context) error
}

type ServiceList []Service
