// Package interfaces defines the various interfaces needed by Carrier.
// e.g. Service, Application etc
package interfaces

import "github.com/suse/carrier/internal/application"

type Service interface {
	Name() string
	Org() string
	Bind(app application.Application) error
	Unbind(app application.Application) error
	Delete() error
}

type ServiceList []Service
