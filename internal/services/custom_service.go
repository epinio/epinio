// TODO: create catalog
// TODO: bind to apps - fill in application package

package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/internal/interfaces"
	"github.com/epinio/epinio/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CustomService is a user defined service.
// Implements the Service interface.
type CustomService struct {
	SecretName string
	OrgName    string
	Service    string
	kubeClient *kubernetes.Cluster
}

// CustomServiceList returns a ServiceList of all available custom Services
func CustomServiceList(kubeClient *kubernetes.Cluster, org string) (interfaces.ServiceList, error) {
	labelSelector := fmt.Sprintf("app.kubernetes.io/name=epinio, epinio.suse.org/organization=%s", org)

	secrets, err := kubeClient.Kubectl.CoreV1().
		Secrets(deployments.WorkloadsDeploymentID).
		List(context.Background(),
			metav1.ListOptions{
				LabelSelector: labelSelector,
			})

	if err != nil {
		return nil, err
	}

	result := interfaces.ServiceList{}

	for _, s := range secrets.Items {
		service := s.ObjectMeta.Labels["epinio.suse.org/service"]
		org := s.ObjectMeta.Labels["epinio.suse.org/organization"]
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

// CustomServiceLookup finds a Custom Service by looking for the relevant Secret.
func CustomServiceLookup(kubeClient *kubernetes.Cluster, org, service string) (interfaces.Service, error) {
	secretName := serviceResourceName(org, service)

	_, err := kubeClient.GetSecret(deployments.WorkloadsDeploymentID, secretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return &CustomService{
		SecretName: secretName,
		OrgName:    org,
		Service:    service,
		kubeClient: kubeClient,
	}, nil
}

// CreateCustomService creates a new custom service from org, name and the
// binding data.
func CreateCustomService(kubeClient *kubernetes.Cluster, name, org string,
	data map[string]string) (interfaces.Service, error) {

	secretName := serviceResourceName(org, name)

	_, err := kubeClient.GetSecret(deployments.WorkloadsDeploymentID, secretName)
	if err == nil {
		return nil, errors.New("Service of this name already exists.")
	}

	// Convert from `string -> string` to the `string -> []byte` expected
	// by kube.
	sdata := make(map[string][]byte)
	for k, v := range data {
		sdata[k] = []byte(v)
	}

	err = kubeClient.CreateLabeledSecret("epinio-workloads",
		secretName, sdata,
		map[string]string{
			"epinio.suse.org/service-type": "custom",
			"epinio.suse.org/service":      name,
			"epinio.suse.org/organization": org,
			"app.kubernetes.io/name":       "epinio",
			// "app.kubernetes.io/version":     cmd.Version
			// FIXME: Importing cmd causes cycle
			// FIXME: Move version info to separate package!
		},
	)
	if err != nil {
		return nil, err
	}
	return &CustomService{
		SecretName: secretName,
		OrgName:    org,
		Service:    name,
		kubeClient: kubeClient,
	}, nil
}

func (s *CustomService) Name() string {
	return s.Service
}

func (s *CustomService) Org() string {
	return s.OrgName
}

func (s *CustomService) GetBinding(appName string) (*corev1.Secret, error) {
	kubeClient := s.kubeClient
	serviceSecret, err := kubeClient.GetSecret(deployments.WorkloadsDeploymentID, s.SecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errors.New("service does not exist")
		}
		return nil, err
	}

	return serviceSecret, nil
}

// DeleteBinding does nothing in the case of custom services because the custom
// service is just a secret which may be re-used later.
func (s *CustomService) DeleteBinding(appName string) error {
	return nil
}

func (s *CustomService) Delete() error {
	return s.kubeClient.DeleteSecret(deployments.WorkloadsDeploymentID, s.SecretName)
}

func (s *CustomService) Status() (string, error) {
	return "Provisioned", nil
}

func (s *CustomService) WaitForProvision() error {
	// Custom services provision instantly. No waiting
	return nil
}

func (s *CustomService) Details() (map[string]string, error) {
	serviceSecret, err := s.kubeClient.GetSecret(deployments.WorkloadsDeploymentID, s.SecretName)
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
