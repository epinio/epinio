// Package interfaces defines the various interfaces needed by Carrier.
// e.g. Service, Application etc
package interfaces

type Service interface {
	Name() string
	Org() string
	Bind(app Application) error
	Unbind(app Application) error
	Delete() error
}

type Application interface {
	Name() string
	Org() string
	Delete() error
	Bind(org, service string) error
}

type ServiceList []Service
type ApplicationList []Application
