package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/interfaces"
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
	app     models.AppRef
	cluster *kubernetes.Cluster
}

func NewWorkload(cluster *kubernetes.Cluster, app models.AppRef) *Workload {
	return &Workload{cluster: cluster, app: app}
}

// Services returns the set of services bound to the application.
func (a *Workload) Services(ctx context.Context) (interfaces.ServiceList, error) {
	deployment, err := a.deployment(ctx)
	if err != nil {
		return nil, err
	}

	var bound = interfaces.ServiceList{}

	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		service, err := services.Lookup(ctx, a.cluster, a.app.Org, volume.Name)
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

		_, err = a.cluster.Kubectl.AppsV1().Deployments(a.app.Org).Update(
			ctx, deployment, metav1.UpdateOptions{})

		return err
	})
}

// UnbindAll dissolves all bindings from the application.
func (a *Workload) UnbindAll(ctx context.Context, cluster *kubernetes.Cluster, svcs []string) error {
	for _, bonded := range svcs {
		bound, err := services.Lookup(ctx, cluster, a.app.Org, bonded)
		if err != nil {
			return err
		}

		err = a.Unbind(ctx, bound)
		if err != nil {
			return err
		}
	}
	return nil
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

		_, err = a.cluster.Kubectl.AppsV1().Deployments(a.app.Org).Update(
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
	return service.DeleteBinding(ctx, a.app.Name, a.app.Org)
}

func (a *Workload) deployment(ctx context.Context) (*appsv1.Deployment, error) {
	return a.cluster.Kubectl.AppsV1().Deployments(a.app.Org).Get(
		ctx, a.app.Name, metav1.GetOptions{},
	)
}

// Bind creates a binding of the service to the application.
func (a *Workload) Bind(ctx context.Context, service interfaces.Service) error {
	bindSecret, err := service.GetBinding(ctx, a.app.Name)
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

		_, err = a.cluster.Kubectl.AppsV1().Deployments(a.app.Org).Update(
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

	app := a.app.App()

	// Query application deployment for stageID and status (ready vs desired replicas)

	deploymentSelector := fmt.Sprintf("app.kubernetes.io/part-of=%s,app.kubernetes.io/name=%s", a.app.Org, a.app.Name)

	deploymentListOptions := metav1.ListOptions{
		LabelSelector: deploymentSelector,
	}

	deployments, err := a.cluster.Kubectl.AppsV1().Deployments(a.app.Org).List(ctx, deploymentListOptions)

	if err != nil {
		app.Status = pkgerrors.Wrap(err, "failed to get Deployment status").Error()
	} else if len(deployments.Items) < 1 {
		app.Status = "0/0"
	} else {
		app.Status = fmt.Sprintf("%d/%d",
			deployments.Items[0].Status.ReadyReplicas,
			deployments.Items[0].Status.Replicas)

		app.StageID = deployments.Items[0].
			Spec.Template.ObjectMeta.Labels["epinio.suse.org/stage-id"]

		app.Active = true
	}

	app.Routes, err = a.cluster.ListIngressRoutes(ctx, app.Organization, app.Name)
	if err != nil {
		app.Routes = []string{err.Error()}
	}

	app.BoundServices = []string{}
	bound, err := a.Services(ctx)
	if err != nil {
		app.BoundServices = append(app.BoundServices, err.Error())
	} else {
		for _, service := range bound {
			app.BoundServices = append(app.BoundServices, service.Name())
		}
	}

	return app, nil
}
