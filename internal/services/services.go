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

	// TODO: Update when CatalogServices become available.
	// At the moment the kind is always CustomService.

	// Catalog services have a `ServiceInstance` here, instead of a
	// secret.  IOW we have to perform two lookups. We could, if we wanted
	// to, create our own secret in parallel for catalog services. Then a
	// single lookup would be good enough here, and in List.

	secretName := serviceSecretName(org, service)

	_, err := kubeClient.GetSecret("carrier-workloads", secretName)
	if err != nil {
		return nil, errors.New("Service does not exist.")
	}

	// Pull information about the service out of the secret.
	// IOW kind (name, org are known from the arguments)
	// We could assert that the data in the secret matches the arguments

	// TODO kind := secret.ObjectMeta.Label["carrier.suse.org/service-type"]

	// -- See also List, see if we can factor structure creation into a
	// common internal function.

	return &CustomService{
		SecretName: secretName,
		Organization:    org,
		Service:    service,
		kubeClient: kubeClient,
	}, nil
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
			Organization:    org,
			Service:    service,
			kubeClient: kubeClient,
		})
	}

	return result, nil
}

func serviceSecretName(org, service string) string {
	return fmt.Sprintf("service.org-%s.svc-%s", org, service)
}

func bindingSecretName(org, service, app string) string {
	return fmt.Sprintf("service.org-%s.svc-%s.app-%s", org, service, app)
}
