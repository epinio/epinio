// Package interfaces defines the various interfaces needed by Epinio.
// e.g. Service, Application etc
package interfaces

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

type Service interface {
	Name() string
	Org() string
	GetBinding(ctx context.Context, appName string) (*corev1.Secret, error)
	DeleteBinding(ctx context.Context, appName, org string) error
	Delete(context.Context) error
	Status(context.Context) (string, error)
	Details(context.Context) (map[string]string, error)
	WaitForProvision(context.Context) error
}

type ServiceList []Service
