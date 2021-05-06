package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/interfaces"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
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
	kubeClient    *kubernetes.Cluster
}

type ApplicationList []Application

type GiteaInterface interface {
	DeleteRepo(org, repo string) error
	CreateOrg(org string) error
}

func (a *Application) Delete(gitea GiteaInterface) error {
	if err := gitea.DeleteRepo(a.Organization, a.Name); err != nil {
		return pkgerrors.Wrap(err, "failed to delete repository")
	}

	err := a.kubeClient.Kubectl.AppsV1().Deployments(a.Organization).
		Delete(context.Background(), a.Name, metav1.DeleteOptions{})

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

		_, err = a.kubeClient.Kubectl.AppsV1().Deployments(a.Organization).Update(
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
	return service.DeleteBinding(a.Name, a.Organization)
}

func (a *Application) deployment() (*appsv1.Deployment, error) {
	return a.kubeClient.Kubectl.AppsV1().Deployments(a.Organization).Get(
		context.Background(), a.Name, metav1.GetOptions{},
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

		_, err = a.kubeClient.Kubectl.AppsV1().Deployments(a.Organization).Update(
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
func Lookup(kubeClient *kubernetes.Cluster, org, lookupApp string) (*Application, error) {
	apps, err := List(kubeClient, org)
	if err != nil {
		return nil, err
	}

	for _, app := range apps {
		if app.Name == lookupApp {
			return &app, nil // It's already "Complete()" by the List call above
		}
	}

	return nil, nil
}

// Delete deletes an application by org and name
func Delete(kubeClient *kubernetes.Cluster, gitea GiteaInterface, org string, app Application) error {
	if len(app.BoundServices) > 0 {
		for _, bonded := range app.BoundServices {
			bound, err := services.Lookup(kubeClient, org, bonded)
			if err != nil {
				return err
			}

			err = app.Unbind(bound)
			if err != nil {
				return err
			}
		}
	}

	err := app.Delete(gitea)
	if err != nil {
		return err
	}

	err = kubeClient.WaitForPodBySelectorMissing(nil,
		app.Organization,
		fmt.Sprintf("app.kubernetes.io/name=%s", app.Name),
		duration.ToDeployment())
	if err != nil {
		return err
	}

	return nil
}

// List returns an ApplicationList of all available applications (in the org)
func List(kubeClient *kubernetes.Cluster, org string) (ApplicationList, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=application,app.kubernetes.io/managed-by=epinio",
	}

	result := ApplicationList{}

	exists, err := organizations.Exists(kubeClient, org)
	if err != nil {
		return result, err
	}
	if !exists {
		return result, fmt.Errorf("organization %s does not exist", org)
	}

	deployments, err := kubeClient.Kubectl.AppsV1().Deployments(org).List(context.Background(), listOptions)
	if err != nil {
		return result, err
	}

	for _, deployment := range deployments.Items {
		appEpinio, err := (&Application{
			Organization: org,
			Name:         deployment.ObjectMeta.Name,
			kubeClient:   kubeClient,
		}).Complete()
		if err != nil {
			return result, err
		}

		result = append(result, *appEpinio)
	}

	return result, nil
}

func (app *Application) Complete() (*Application, error) {
	var err error
	app.Status, err = app.kubeClient.DeploymentStatus(
		app.Organization,
		fmt.Sprintf("app.kubernetes.io/part-of=%s,app.kubernetes.io/name=%s",
			app.Organization, app.Name))
	if err != nil {
		app.Status = err.Error()
	}

	app.Routes, err = app.kubeClient.ListIngressRoutes(
		app.Organization, app.Name)
	if err != nil {
		app.Routes = []string{err.Error()}
	}

	app.BoundServices = []string{}
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
