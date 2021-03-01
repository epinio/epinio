// TODO: create catalog
// TODO: bind to apps - fill in application package

package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/suse/carrier/deployments"
	"github.com/suse/carrier/internal/application"
	"github.com/suse/carrier/internal/interfaces"
	"github.com/suse/carrier/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (s *CustomService) Bind(app application.Application) error {
	kubeClient := s.kubeClient
	serviceSecret, err := kubeClient.GetSecret("carrier-workloads", s.SecretName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return errors.New("Service does not exist.")
		}
		return err
	}

	// TODO: Move this code to the Application structure
	deployment, err := kubeClient.Kubectl.AppsV1().Deployments(deployments.WorkloadsDeploymentID).Get(
		context.Background(),
		fmt.Sprintf("%s.%s", s.OrgName, app.Name),
		metav1.GetOptions{},
	)
	if err != nil {
		return err
	}

	volumes := deployment.Spec.Template.Spec.Volumes

	for _, volume := range volumes {
		if volume.Name == s.Service {
			return errors.New("service already bound")
		}
	}

	volumes = append(volumes, corev1.Volume{
		Name: s.Service,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: serviceSecret.Name,
			},
		},
	})
	// TODO: Iterate over containers and find the one matching the app name
	volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      s.Service,
		ReadOnly:  true,
		MountPath: fmt.Sprintf("/services/%s", s.Service),
	})

	deployment.Spec.Template.Spec.Volumes = volumes
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	_, err = kubeClient.Kubectl.AppsV1().Deployments(deployments.WorkloadsDeploymentID).Update(
		context.Background(),
		deployment,
		v1.UpdateOptions{},
	)
	return err
}

func (s *CustomService) Unbind(app application.Application) error {
	// TODO remove custom service binding to app
	return nil
}

func (s *CustomService) Delete() error {
	return s.kubeClient.DeleteSecret("carrier-workloads", s.SecretName)
}
