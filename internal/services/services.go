// Package services package incapsulates all the functionality around Carrier services
package services

import (
	"errors"
	"fmt"

	"github.com/suse/carrier/internal/interfaces"
	"github.com/suse/carrier/kubernetes"
)

// Lookup locates a Service by org and name
func Lookup(kubeClient *kubernetes.Cluster, org, service string) (interfaces.Service, error) {
	serviceInstance, err := CustomServiceLookup(kubeClient, org, service)
	if err != nil {
		return nil, err
	}
	if serviceInstance != nil {
		return serviceInstance, nil
	}

	serviceInstance, err = CatalogServiceLookup(kubeClient, org, service)
	if err != nil {
		return nil, err
	}
	if serviceInstance != nil {
		return serviceInstance, nil
	}

	return nil, errors.New("service not found")
}

// List returns a ServiceList of all available Services
func List(kubeClient *kubernetes.Cluster, org string) (interfaces.ServiceList, error) {

	customServices, err := CustomServiceList(kubeClient, org)
	if err != nil {
		return nil, err
	}

	catalogServices, err := CatalogServiceList(kubeClient, org)
	if err != nil {
		return nil, err
	}

	return append(customServices, catalogServices...), nil
}

func serviceResourceName(org, service string) string {
	return fmt.Sprintf("service.org-%s.svc-%s", org, service)
}

func bindingResourceName(org, service, app string) string {
	return fmt.Sprintf("service.org-%s.svc-%s.app-%s", org, service, app)
}
