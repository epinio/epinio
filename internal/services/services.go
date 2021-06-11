// Package services incapsulates all the functionality around Epinio services
package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/interfaces"
)

// Lookup locates a Service by org and name
func Lookup(ctx context.Context, kubeClient *kubernetes.Cluster, org, service string) (interfaces.Service, error) {
	serviceInstance, err := CustomServiceLookup(ctx, kubeClient, org, service)
	if err != nil {
		return nil, err
	}
	if serviceInstance != nil {
		return serviceInstance, nil
	}

	serviceInstance, err = CatalogServiceLookup(ctx, kubeClient, org, service)
	if err != nil {
		return nil, err
	}
	if serviceInstance != nil {
		return serviceInstance, nil
	}

	return nil, errors.New("service not found")
}

// List returns a ServiceList of all available Services
func List(ctx context.Context, kubeClient *kubernetes.Cluster, org string) (interfaces.ServiceList, error) {

	customServices, err := CustomServiceList(ctx, kubeClient, org)
	if err != nil {
		return nil, err
	}

	catalogServices, err := CatalogServiceList(ctx, kubeClient, org)
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
