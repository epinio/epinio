// Package services encapsulates all the functionality around Epinio
// services, catalog-based and custom
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

	return nil, errors.New("service not found")
}

// List returns a ServiceList of all available Services
func List(ctx context.Context, kubeClient *kubernetes.Cluster, org string) (interfaces.ServiceList, error) {
	return CustomServiceList(ctx, kubeClient, org)
}

// serviceResourceName returns a name for a kube service resource
// representing the org and service
func serviceResourceName(org, service string) string {
	return fmt.Sprintf("service.org-%s.svc-%s", org, service)
}

// bindingResourceName returns a name for a kube service binding
// resource representing the org, service, and application
func bindingResourceName(org, service, app string) string {
	return fmt.Sprintf("service.org-%s.svc-%s.app-%s", org, service, app)
}
