package application

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/sdk/gitea"
	"github.com/suse/carrier/deployments"
	"github.com/suse/carrier/internal/interfaces"
	"github.com/suse/carrier/kubernetes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Application manages applications.
// Implements the Application interface.
type Application struct {
	Name         string
	Organization string
	giteaClient  *gitea.Client
	kubeClient   *kubernetes.Cluster
}

type ApplicationList []Application

func (a *Application) Delete() error {
	// TODO: delete application
	// NOTE: has to do the things client does, without UI messages!
	// Hide the things (repo, hooks, whatnot, ...) from the user.
	return nil
}

func (a *Application) Bind(service interfaces.Service) error {
	bindSecret, err := service.GetBinding(a.Name)
	if err != nil {
		return err
	}

	deployment, err := a.kubeClient.Kubectl.AppsV1().Deployments(deployments.WorkloadsDeploymentID).Get(
		context.Background(),
		fmt.Sprintf("%s.%s", a.Organization, a.Name),
		metav1.GetOptions{},
	)
	if err != nil {
		return err
	}

	volumes := deployment.Spec.Template.Spec.Volumes

	for _, volume := range volumes {
		if volume.Name == service.Name() {
			return errors.New("service already bound")
		}
	}

	volumes = append(volumes, corev1.Volume{
		Name: service.Name(),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: bindSecret.Name,
			},
		},
	})
	// TODO: Iterate over containers and find the one matching the app name
	volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      service.Name(),
		ReadOnly:  true,
		MountPath: fmt.Sprintf("/services/%s", service.Name()),
	})

	deployment.Spec.Template.Spec.Volumes = volumes
	deployment.Spec.Template.Spec.Containers[0].VolumeMounts = volumeMounts

	_, err = a.kubeClient.Kubectl.AppsV1().Deployments(deployments.WorkloadsDeploymentID).Update(
		context.Background(),
		deployment,
		metav1.UpdateOptions{},
	)
	return err
}

// Lookup locates an Application by org and name
func Lookup(
	kubeClient *kubernetes.Cluster,
	giteaClient *gitea.Client,
	org, app string) (Application, error) {

	apps, _, err := giteaClient.ListOrgRepos(org, gitea.ListOrgReposOptions{})
	if err != nil {
		return Application{}, err
	}

	for _, anApp := range apps {
		if anApp.Name == app {
			return Application{
				Organization: org,
				Name:         app,
				kubeClient:   kubeClient,
				giteaClient:  giteaClient,
			}, nil
		}
	}

	return Application{}, errors.New("Application not found")
}

// List returns an ApplicationList of all available applications (in the org)
func List(
	kubeClient *kubernetes.Cluster,
	giteaClient *gitea.Client,
	org string) (ApplicationList, error) {

	apps, _, err := giteaClient.ListOrgRepos(org, gitea.ListOrgReposOptions{})
	if err != nil {
		return nil, err
	}

	result := ApplicationList{}

	for _, app := range apps {
		result = append(result, Application{
			Organization: org,
			Name:         app.Name,
			kubeClient:   kubeClient,
			giteaClient:  giteaClient,
		})
	}

	return result, nil
}
