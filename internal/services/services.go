// Package services encapsulates all the functionality around Epinio services
package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ServiceList []*Service

// Service contains the information needed for Epinio to address a specific service.
type Service struct {
	SecretName string
	OrgName    string
	Service    string
	Username   string
	kubeClient *kubernetes.Cluster
}

// Lookup locates a Service by org and name. It finds the Service
// instance by looking for the relevant Secret.
func Lookup(ctx context.Context, kubeClient *kubernetes.Cluster, org, service string) (*Service, error) {
	// TODO 844 inline

	secretName := serviceResourceName(org, service)

	s, err := kubeClient.GetSecret(ctx, org, secretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errors.New("service not found")
		}
		return nil, err
	}
	username := s.ObjectMeta.Labels["app.kubernetes.io/created-by"]

	return &Service{
		SecretName: secretName,
		OrgName:    org,
		Service:    service,
		kubeClient: kubeClient,
		Username:   username,
	}, nil
}

// List returns a ServiceList of all available Services
func List(ctx context.Context, kubeClient *kubernetes.Cluster, org string) (ServiceList, error) {
	labelSelector := fmt.Sprintf("app.kubernetes.io/name=epinio, epinio.suse.org/namespace=%s", org)

	secrets, err := kubeClient.Kubectl.CoreV1().
		Secrets(org).List(ctx,
		metav1.ListOptions{
			LabelSelector: labelSelector,
		})

	if err != nil {
		return nil, err
	}

	result := ServiceList{}

	for _, s := range secrets.Items {
		service := s.ObjectMeta.Labels["epinio.suse.org/service"]
		org := s.ObjectMeta.Labels["epinio.suse.org/namespace"]
		username := s.ObjectMeta.Labels["app.kubernetes.io/created-by"]

		secretName := s.ObjectMeta.Name

		result = append(result, &Service{
			SecretName: secretName,
			OrgName:    org,
			Service:    service,
			kubeClient: kubeClient,
			Username:   username,
		})
	}

	return result, nil
}

// CreateService creates a new  service instance from org,
// name, and a map of parameters.
func CreateService(ctx context.Context, kubeClient *kubernetes.Cluster, name, org, username string,
	data map[string]string) (*Service, error) {

	secretName := serviceResourceName(org, name)

	_, err := kubeClient.GetSecret(ctx, org, secretName)
	if err == nil {
		return nil, errors.New("Service of this name already exists.")
	}

	// Convert from `string -> string` to the `string -> []byte` expected
	// by kube.
	sdata := make(map[string][]byte)
	for k, v := range data {
		sdata[k] = []byte(v)
	}

	err = kubeClient.CreateLabeledSecret(ctx, org, secretName, sdata,
		map[string]string{
			// "epinio.suse.org/service-type": "custom",
			"epinio.suse.org/service":      name,
			"epinio.suse.org/namespace":    org,
			"app.kubernetes.io/name":       "epinio",
			"app.kubernetes.io/created-by": username,
			// "app.kubernetes.io/version":     cmd.Version
			// FIXME: Importing cmd causes cycle
			// FIXME: Move version info to separate package!
		},
	)
	if err != nil {
		return nil, err
	}
	return &Service{
		SecretName: secretName,
		OrgName:    org,
		Service:    name,
		kubeClient: kubeClient,
	}, nil
}

// Name returns the service instance's name
func (s *Service) Name() string {
	return s.Service
}

// User returns the service's username
func (s *Service) User() string {
	return s.Username
}

// Org returns the service instance's organization
func (s *Service) Org() string {
	return s.OrgName
}

// GetBinding returns the secret representing the instance's binding
// to the application. This is actually the instance's secret itself,
// independent of the application.
func (s *Service) GetBinding(ctx context.Context, appName string, _ string) (*corev1.Secret, error) {
	kubeClient := s.kubeClient
	serviceSecret, err := kubeClient.GetSecret(ctx, s.OrgName, s.SecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errors.New("service does not exist")
		}
		return nil, err
	}

	return serviceSecret, nil
}

// Delete destroys the service instance, i.e. its underlying secret
// holding the instance's parameters
func (s *Service) Delete(ctx context.Context) error {
	return s.kubeClient.DeleteSecret(ctx, s.OrgName, s.SecretName)
}

// Details returns the service instance's configuration.
// I.e. the parameter data.
func (s *Service) Details(ctx context.Context) (map[string]string, error) {
	serviceSecret, err := s.kubeClient.GetSecret(ctx, s.OrgName, s.SecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errors.New("service does not exist")
		}
		return nil, err
	}

	details := map[string]string{}

	for k, v := range serviceSecret.Data {
		details[k] = string(v)
	}

	return details, nil
}

// serviceResourceName returns a name for a kube service resource
// representing the org and service
func serviceResourceName(org, service string) string {
	return fmt.Sprintf("service.org-%s.svc-%s", org, service)
}
