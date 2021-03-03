// Package services package incapsulates all the functionality around Carrier services
package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/suse/carrier/internal/interfaces"
	"github.com/suse/carrier/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Filter for carrier secrets referencing the targeted organization
	// TODO: We either need to calls, for secrets of custom services, and
	// service instances of catalog servers, or we construct a secret for
	// catalog services also, in parallel to the instance.

	labelSelector := fmt.Sprintf("app.kubernetes.io/name=carrier, carrier.suse.org/organization=%s", org)

	secrets, err := kubeClient.Kubectl.CoreV1().
		Secrets("carrier-workloads").
		List(context.Background(),
			metav1.ListOptions{
				LabelSelector: labelSelector,
			})

	if err != nil {
		return nil, err
	}

	result := interfaces.ServiceList{}

	for _, s := range secrets.Items {
		// TODO kind/type
		service := s.ObjectMeta.Labels["carrier.suse.org/service"]
		org := s.ObjectMeta.Labels["carrier.suse.org/organization"]
		secretName := s.ObjectMeta.Name

		result = append(result, &CustomService{
			SecretName: secretName,
			OrgName:    org,
			Service:    service,
			kubeClient: kubeClient,
		})
	}

	return result, nil
}

func serviceResourceName(org, service string) string {
	return fmt.Sprintf("service.org-%s.svc-%s", org, service)
}

func bindingResourceName(org, service, app string) string {
	return fmt.Sprintf("service.org-%s.svc-%s.app-%s", org, service, app)
}
