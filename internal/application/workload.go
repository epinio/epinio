package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/interfaces"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	pkgerrors "github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

// Workload manages applications that are deployed. It provides workload
// (deployments) specific actions for the application model.
type Workload struct {
	// embedding this struct only to stay compatible with existing code,
	// otherwise it would be explicit: a workload belongs to an app
	*models.App
	cluster *kubernetes.Cluster
}

func NewWorkload(cluster *kubernetes.Cluster, app *models.App) *Workload {
	return &Workload{cluster: cluster, App: app}
}

// Lookup locates an application's workload by org and name
func Lookup(ctx context.Context, cluster *kubernetes.Cluster, org, lookupApp string) (*models.App, error) {
	apps, err := List(ctx, cluster, org)
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

// List returns an list of all available applications (in the org)
func List(ctx context.Context, cluster *kubernetes.Cluster, org string) (models.AppList, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=application,app.kubernetes.io/managed-by=epinio",
	}

	result := models.AppList{}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return result, err
	}
	if !exists {
		return result, fmt.Errorf("organization %s does not exist", org)
	}

	deployments, err := cluster.Kubectl.AppsV1().Deployments(org).List(ctx, listOptions)
	if err != nil {
		return result, err
	}

	for _, deployment := range deployments.Items {
		w := NewWorkload(cluster, &models.App{
			Organization: org,
			Name:         deployment.ObjectMeta.Name,
		})
		appEpinio, err := w.Complete(ctx)
		if err != nil {
			return result, err
		}

		result = append(result, *appEpinio)
	}

	return result, nil
}

// Delete a workload (repo, deployments, ingress, services)
func (a *Workload) Delete(ctx context.Context, gitea GiteaInterface) error {
	if err := gitea.DeleteRepo(a.Organization, a.Name); err != nil {
		return pkgerrors.Wrap(err, "failed to delete repository")
	}

	err := a.cluster.Kubectl.AppsV1().Deployments(a.Organization).
		Delete(ctx, a.Name, metav1.DeleteOptions{})
	if err != nil {
		return pkgerrors.Wrap(err, "failed to delete application deployment")
	}

	err = a.cluster.Kubectl.ExtensionsV1beta1().Ingresses(a.Organization).
		Delete(ctx, a.Name, metav1.DeleteOptions{})
	if err != nil {
		return pkgerrors.Wrap(err, "failed to delete application ingress")
	}

	err = a.cluster.Kubectl.CoreV1().Services(a.Organization).
		Delete(ctx, a.Name, metav1.DeleteOptions{})
	if err != nil {
		return pkgerrors.Wrap(err, "failed to delete application service")
	}

	return nil
}

// Services returns the set of services bound to the application.
func (a *Workload) Services(ctx context.Context) (interfaces.ServiceList, error) {
	deployment, err := a.deployment(ctx)
	if err != nil {
		return nil, err
	}

	var bound = interfaces.ServiceList{}

	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		service, err := services.Lookup(ctx, a.cluster, a.Organization, volume.Name)
		if err != nil {
			return nil, err
		}
		bound = append(bound, service)
	}

	return bound, nil
}

// Scale should be used to change the number of instances (replicas) on the
// application Deployment.
func (a *Workload) Scale(ctx context.Context, instances int32) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of Deployment before attempting update
		// RetryOnConflict uses exponential backoff to avoid exhausting the apiserver
		deployment, err := a.deployment(ctx)
		if err != nil {
			return err
		}

		deployment.Spec.Replicas = &instances

		_, err = a.cluster.Kubectl.AppsV1().Deployments(a.Organization).Update(
			ctx, deployment, metav1.UpdateOptions{})

		return err
	})
}

// Unbind dissolves the binding of the service to the application.
func (a *Workload) Unbind(ctx context.Context, service interfaces.Service) error {
	for {
		deployment, err := a.deployment(ctx)
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

		_, err = a.cluster.Kubectl.AppsV1().Deployments(a.Organization).Update(
			ctx,
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
	return service.DeleteBinding(ctx, a.Name, a.Organization)
}

func (a *Workload) deployment(ctx context.Context) (*appsv1.Deployment, error) {
	return a.cluster.Kubectl.AppsV1().Deployments(a.Organization).Get(
		ctx, a.Name, metav1.GetOptions{},
	)
}

// Bind creates a binding of the service to the application.
func (a *Workload) Bind(ctx context.Context, service interfaces.Service) error {
	bindSecret, err := service.GetBinding(ctx, a.Name)
	if err != nil {
		return err
	}

	for {
		deployment, err := a.deployment(ctx)
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

		_, err = a.cluster.Kubectl.AppsV1().Deployments(a.Organization).Update(
			ctx,
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

// Complete fills all fields of a workload with values from the cluster
func (a *Workload) Complete(ctx context.Context) (*models.App, error) {
	var err error

	selector := fmt.Sprintf("app.kubernetes.io/component=application,app.kubernetes.io/managed-by=epinio,app.kubernetes.io/name=%s",
		a.Name)

	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	pods, err := a.cluster.Kubectl.CoreV1().Pods(a.Organization).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	a.StageID = pods.Items[0].ObjectMeta.Labels["epinio.suse.org/stage-id"]

	a.Status, err = a.cluster.DeploymentStatus(ctx,
		a.Organization,
		fmt.Sprintf("app.kubernetes.io/part-of=%s,app.kubernetes.io/name=%s",
			a.Organization, a.Name))
	if err != nil {
		a.Status = err.Error()
	}

	a.Routes, err = a.cluster.ListIngressRoutes(ctx,
		a.Organization, a.Name)
	if err != nil {
		a.Routes = []string{err.Error()}
	}

	a.BoundServices = []string{}
	bound, err := a.Services(ctx)
	if err != nil {
		a.BoundServices = append(a.BoundServices, err.Error())
	} else {
		for _, service := range bound {
			a.BoundServices = append(a.BoundServices, service.Name())
		}
	}

	return a.App, nil
}
