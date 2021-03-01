// TODO: create catalog
// TODO: bind to apps - fill in application package

package services

import (
	"errors"
	"github.com/suse/carrier/internal/interfaces"
	"github.com/suse/carrier/kubernetes"
)

// CustomService is a user defined service.
// Implements the Service interface.
type CustomService struct {
	SecretName string
	OrgName    string
	Service    string
	kubeClient *kubernetes.Cluster
}

// CreateCustomService creates a new custom service from org, name and the
// binding data.
func CreateCustomService(kubeClient *kubernetes.Cluster, name, org string,
	data map[string]string) (interfaces.Service, error) {

	secretName := serviceSecretName(org, name)

	_, err := kubeClient.GetSecret("carrier-workloads", secretName)
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

func (s *CustomService) Bind(app interfaces.Application) error {
	kubeClient := s.kubeClient
	serviceSecret, err := kubeClient.GetSecret("carrier-workloads", s.SecretName)

	if err == nil {
		return errors.New("Service does not exist.")
	}

	secretName := bindingSecretName(s.OrgName, s.Service, app.Name())

	_, err = kubeClient.GetSecret("carrier-workloads", secretName)
	if err == nil {
		return errors.New("Binding already exists.")
	}

	err = kubeClient.CreateLabeledSecret("carrier-workloads",
		secretName, serviceSecret.Data,
		map[string]string{
			"carrier.suse.org/service-type": "custom",
			"carrier.suse.org/service":      s.Service,
			"carrier.suse.org/organization": s.OrgName,
			"carrier.suse.org/application":  app.Name(),
			"app.kubernetes.io/name":        "carrier",
			// "app.kubernetes.io/version":     cmd.Version
			// FIXME: Importing cmd causes cycle
			// FIXME: Move version info to separate package!
		},
	)

	if err != nil {
		return err
	}

	// Alter: Pass the created kube secret directly?
	return app.Bind(s.OrgName, s.Service)
}

func (s *CustomService) Unbind(app interfaces.Application) error {
	return nil
}

func (s *CustomService) Delete() error {
	return s.kubeClient.DeleteSecret("carrier-workloads", s.SecretName)
}
