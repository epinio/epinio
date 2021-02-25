// Package interfaces defines the various interfaces needed by Carrier.
// e.g. Service, Application etc
package interfaces

type Service interface {
	Name() string
	Bind() error
	Unbind() error
	Delete() error
}

type Application interface {
	Name() string
	Delete() error
}

type ServiceList []Service
