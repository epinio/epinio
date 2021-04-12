package application

import (
	"context"
	"errors"
	"fmt"

	"code.gitea.io/sdk/gitea"
	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/internal/interfaces"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/kubernetes"
	pkgerrors "github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Application manages applications.
// Implements the Application interface.
type Application struct {
	Name          string
	Organization  string
	Status        string
	Routes        []string
	BoundServices []string
	giteaClient   *gitea.Client
	kubeClient    *kubernetes.Cluster
}

type ApplicationList []Application

func (a *Application) Delete() error {
	_, err := a.giteaClient.DeleteRepo(a.Organization, a.Name)

	if err != nil {
		return pkgerrors.Wrap(err, "failed to delete repository")
	}

	err = a.kubeClient.Kubectl.AppsV1().Deployments(deployments.WorkloadsDeploymentID).
		Delete(context.Background(), fmt.Sprintf("%s.%s", a.Organization, a.Name), metav1.DeleteOptions{})

	if err != nil {
		return pkgerrors.Wrap(err, "failed to delete application deployment")
	}

	return nil
}

// Services returns the set of services bound to the application.
func (a *Application) Services() (interfaces.ServiceList, error) {
	deployment, err := a.deployment()
	if err != nil {
		return nil, err
	}

	var bound = interfaces.ServiceList{}

	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		service, err := services.Lookup(a.kubeClient, a.Organization, volume.Name)
		if err != nil {
			return nil, err
		}
		bound = append(bound, service)
	}

	return bound, nil
}

// Unbind dissolves the binding of the service to the application.
func (a *Application) Unbind(service interfaces.Service) error {
	for {
		deployment, err := a.deployment()
		if err != nil {
			return err
		}

		volumes := deployment.Spec.Template.Spec.Volumes
		newVolumes := []corev1.Volume{}
		found := false
		for _, volume := range volumes {
			if volume.Name == service.Name() {
				found = true
			} else {
				newVolumes = append(newVolumes, volume)
			}
		}
		if !found {
			return errors.New("service is not bound to the application")
		}

		// TODO: Iterate over containers and find the one matching the app name
		volumeMounts := deployment.Spec.Template.Spec.Containers[0].VolumeMounts
		newVolumeMounts := []corev1.VolumeMount{}
		found = false
		for _, mount := range volumeMounts {
			if mount.Name == service.Name() {
				found = true
			} else {
				newVolumeMounts = append(newVolumeMounts, mount)
			}
		}
		if !found {
			return errors.New("service is not bound to the application")
		}

		deployment.Spec.Template.Spec.Volumes = newVolumes
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = newVolumeMounts

		_, err = a.kubeClient.Kubectl.AppsV1().Deployments(deployments.WorkloadsDeploymentID).Update(
			context.Background(),
			deployment,
			metav1.UpdateOptions{},
		)
		if err == nil {
			break
		}
		if !apierrors.IsConflict(err) {
			return err
		}

		// Found a conflict. Try again from the beginning.
	}

	// delete binding - DeleteBinding(a.Name)
	return service.DeleteBinding(a.Name)
}

func (a *Application) deployment() (*appsv1.Deployment, error) {
	return a.kubeClient.Kubectl.AppsV1().Deployments(deployments.WorkloadsDeploymentID).Get(
		context.Background(),
		fmt.Sprintf("%s.%s", a.Organization, a.Name),
		metav1.GetOptions{},
	)
}

// Bind creates a binding of the service to the application.
func (a *Application) Bind(service interfaces.Service) error {
	bindSecret, err := service.GetBinding(a.Name)
	if err != nil {
		return err
	}

	for {
		deployment, err := a.deployment()
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

		if err == nil {
			break
		}
		if !apierrors.IsConflict(err) {
			return err
		}

		// Found a conflict. Try again from the beginning.
	}

	return nil
}

// Lookup locates an Application by org and name
func Lookup(
	kubeClient *kubernetes.Cluster,
	giteaClient *gitea.Client,
	org, app string) (*Application, error) {

	apps, _, err := giteaClient.ListOrgRepos(org, gitea.ListOrgReposOptions{})
	if err != nil {
		return nil, err
	}

	for _, anApp := range apps {
		if anApp.Name == app {
			app, err := (&Application{
				Organization: org,
				Name:         app,
				kubeClient:   kubeClient,
				giteaClient:  giteaClient,
			}).Complete()
			if err != nil {
				return nil, err
			}
			return app, nil
		}
	}

	return nil, nil
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
		appEpinio, err := (&Application{
			Organization: org,
			Name:         app.Name,
			kubeClient:   kubeClient,
			giteaClient:  giteaClient,
		}).Complete()
		if err != nil {
			return nil, err
		}

		result = append(result, *appEpinio)
	}

	return result, nil
}

func (app *Application) Complete() (*Application, error) {
	var err error
	app.Status, err = app.kubeClient.DeploymentStatus(
		deployments.WorkloadsDeploymentID,
		fmt.Sprintf("app.kubernetes.io/part-of=%s,app.kubernetes.io/name=%s",
			app.Organization, app.Name))
	if err != nil {
		app.Status = err.Error()
	}

	app.Routes, err = app.kubeClient.ListIngressRoutes(
		deployments.WorkloadsDeploymentID,
		app.Name)
	if err != nil {
		app.Routes = []string{err.Error()}
	}

	bound, err := app.Services()
	if err != nil {
		app.BoundServices = append(app.BoundServices, err.Error())
	} else {
		for _, service := range bound {
			app.BoundServices = append(app.BoundServices, service.Name())
		}
	}

	return app, nil
}
