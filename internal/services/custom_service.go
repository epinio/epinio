// TODO: create catalog
// TODO: bind to apps - fill in application package

package services

import (
	"errors"

	"github.com/suse/carrier/deployments"
	"github.com/suse/carrier/internal/interfaces"
	"github.com/suse/carrier/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// CustomService is a user defined service.
// Implements the Service interface.
type CustomService struct {
	SecretName string
	OrgName    string
	Service    string
	kubeClient *kubernetes.Cluster
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

	err = kubeClient.CreateLabeledSecret("carrier-workloads",
		secretName, sdata,
		map[string]string{
			"carrier.suse.org/service-type": "custom",
			"carrier.suse.org/service":      name,
			"carrier.suse.org/organization": org,
			"app.kubernetes.io/name":        "carrier",
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
