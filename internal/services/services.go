// Package services package incapsulates all the functionality around Carrier services
package services

import (
	"github.com/suse/carrier/internal/interfaces"
)

// List returns a ServiceList of all available Services
func List() (interfaces.ServiceList, error) {
	return interfaces.ServiceList{}, nil
}
