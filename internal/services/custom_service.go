
// TODO: API carrier client argument - Required for Create
// TODO: API error return - For Create
// TODO: paas/client.go - Use CreateCustomService
// TODO: paas/client - Delegate to services package in general
// TODO: create catalog
// TODO: bind to apps - fill in application package

package services

import (
	"github.com/suse/carrier/internal/interfaces"
)

// CustomService is a use defined service.
// Implements the Service interface.
type CustomService struct{}

func CreateCustomService(name, org string, data map[string]string) interfaces.Service {
	secretName := serviceName(org, name)

	_, err = c.kubeClient.GetSecret("carrier-workloads", secretName)
	if err == nil {
		return error.New("Service of this name already exists.")
	}

	err = c.kubeClient.CreateLabeledSecret("carrier-workloads",
		secretName, data,
		map[string]string{
			"carrier.suse.org/service-type": "custom",
			"carrier.suse.org/service":      name,
			"carrier.suse.org/organization": c.config.Org,
			"app.kubernetes.io/name":        "carrier",
			// "app.kubernetes.io/version":     cmd.Version
			// FIXME: Importing cmd causes cycle
			// FIXME: Move version info to separate package!
		},
	)
	if err != nil {
		return nil
	}
	return &CustomService{}
}

func (s *CustomService) Bind() error {
	return nil
}

func (s *CustomService) Unbind() error {
	return nil
}

func (s *CustomService) Delete() error {
	return nil
}

func serviceName(org, service string) string {
	return fmt.Sprintf("service.org-%s.svc-%s", org, service)
}
