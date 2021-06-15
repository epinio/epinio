// Package application has actor functions that deal with application workloads
// on k8s
package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/interfaces"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	pkgerrors "github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	appv1beta1 "sigs.k8s.io/application/api/v1beta1"
)

// Application manages applications.
// Implements the Application interface.
type Application struct {
	Name          string
	Organization  string
	Status        string
	StageID       string
	Routes        []string
	BoundServices []string
	cluster       *kubernetes.Cluster
}

type ApplicationList []Application

type GiteaInterface interface {
	DeleteRepo(org, repo string) error
}

func Create(ctx context.Context, cluster *kubernetes.Cluster, app *models.App) error {
	cs, err := dynamic.NewForConfig(cluster.RestConfig)
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    "app.k8s.io",
		Version:  "v1beta1",
		Resource: "applications",
	}
	client := cs.Resource(gvr)

	obj := &appv1beta1.Application{
		Spec: appv1beta1.ApplicationSpec{
			Descriptor: appv1beta1.Descriptor{
				Type:   "epinio-workload",
				Owners: []appv1beta1.ContactData{{Name: app.Org}},
			},
		},
	}

	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}
	us := &unstructured.Unstructured{Object: u}
	us.SetAPIVersion("app.k8s.io/v1beta1")
	us.SetKind("Application")
	us.SetName(app.Name)

	_, err = client.Namespace(app.Org).Create(ctx, us, metav1.CreateOptions{})
	return err
}

func (a *Application) Delete(ctx context.Context, gitea GiteaInterface) error {
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
func (a *Application) Services(ctx context.Context) (interfaces.ServiceList, error) {
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
func (a *Application) Scale(ctx context.Context, instances int32) error {
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
func (a *Application) Unbind(ctx context.Context, service interfaces.Service) error {
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

func (a *Application) deployment(ctx context.Context) (*appsv1.Deployment, error) {
	return a.cluster.Kubectl.AppsV1().Deployments(a.Organization).Get(
		ctx, a.Name, metav1.GetOptions{},
	)
}

// Bind creates a binding of the service to the application.
func (a *Application) Bind(ctx context.Context, service interfaces.Service) error {
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

// Lookup locates an Application by org and name
func Lookup(ctx context.Context, cluster *kubernetes.Cluster, org, lookupApp string) (*Application, error) {
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

// Delete deletes an application by org and name
func Delete(ctx context.Context, cluster *kubernetes.Cluster, gitea GiteaInterface, org string, app Application) error {
	if len(app.BoundServices) > 0 {
		for _, bonded := range app.BoundServices {
			bound, err := services.Lookup(ctx, cluster, org, bonded)
			if err != nil {
				return err
			}

			err = app.Unbind(ctx, bound)
			if err != nil {
				return err
			}
		}
	}

	err := app.Delete(ctx, gitea)
	if err != nil {
		return err
	}

	err = Unstage(ctx, app.Name, app.Organization, "")
	if err != nil {
		return err
	}

	err = cluster.WaitForPodBySelectorMissing(ctx, nil,
		app.Organization,
		fmt.Sprintf("app.kubernetes.io/name=%s", app.Name),
		duration.ToDeployment())
	if err != nil {
		return err
	}

	return nil
}

// List returns an ApplicationList of all available applications (in the org)
func List(ctx context.Context, cluster *kubernetes.Cluster, org string) (ApplicationList, error) {
	listOptions := metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=application,app.kubernetes.io/managed-by=epinio",
	}

	result := ApplicationList{}

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
		appEpinio, err := (&Application{
			Organization: org,
			Name:         deployment.ObjectMeta.Name,
			cluster:      cluster,
		}).Complete(ctx)
		if err != nil {
			return result, err
		}

		result = append(result, *appEpinio)
	}

	return result, nil
}

func (app *Application) Complete(ctx context.Context) (*Application, error) {
	var err error

	selector := fmt.Sprintf("app.kubernetes.io/component=application,app.kubernetes.io/managed-by=epinio,app.kubernetes.io/name=%s",
		app.Name)

	listOptions := metav1.ListOptions{
		LabelSelector: selector,
	}

	pods, err := app.cluster.Kubectl.CoreV1().Pods(app.Organization).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	app.StageID = pods.Items[0].ObjectMeta.Labels["epinio.suse.org/stage-id"]

	app.Status, err = app.cluster.DeploymentStatus(ctx,
		app.Organization,
		fmt.Sprintf("app.kubernetes.io/part-of=%s,app.kubernetes.io/name=%s",
			app.Organization, app.Name))
	if err != nil {
		app.Status = err.Error()
	}

	app.Routes, err = app.cluster.ListIngressRoutes(ctx,
		app.Organization, app.Name)
	if err != nil {
		app.Routes = []string{err.Error()}
	}

	app.BoundServices = []string{}
	bound, err := app.Services(ctx)
	if err != nil {
		app.BoundServices = append(app.BoundServices, err.Error())
	} else {
		for _, service := range bound {
			app.BoundServices = append(app.BoundServices, service.Name())
		}
	}

	return app, nil
}

// Implement the Sort interface for application slices

func (al ApplicationList) Len() int {
	return len(al)
}

func (al ApplicationList) Swap(i, j int) {
	al[i], al[j] = al[j], al[i]
}

func (al ApplicationList) Less(i, j int) bool {
	return al[i].Name < al[j].Name
}

// Unstage deletes either all PipelineRuns of the named application, or all but the current.
func Unstage(ctx context.Context, app, org, stageIdCurrent string) error {
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return err
	}

	cs, err := versioned.NewForConfig(cluster.RestConfig)
	if err != nil {
		return err
	}

	client := cs.TektonV1beta1().PipelineRuns(deployments.TektonStagingNamespace)

	l, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s",
			app, org),
	})
	if err != nil {
		return err
	}

	for _, pr := range l.Items {
		if stageIdCurrent != "" && stageIdCurrent == pr.ObjectMeta.Name {
			continue
		}

		err := client.Delete(ctx, pr.ObjectMeta.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// Logs method writes log lines to the specified logChan. The caller can stop
// the logging with the ctx cancelFunc. It's also the callers responsibility
// to close the logChan when done.
// When stageID is an empty string, no staging logs are returned. If it is set,
// then only logs from that staging process are returned.
func Logs(ctx context.Context, logChan chan tailer.ContainerLogLine, wg *sync.WaitGroup, cluster *kubernetes.Cluster, follow bool, app, stageID, org string) error {
	selector := labels.NewSelector()

	var selectors [][]string
	if stageID == "" {
		selectors = [][]string{
			{"app.kubernetes.io/component", "application"},
			{"app.kubernetes.io/managed-by", "epinio"},
			{"app.kubernetes.io/part-of", org},
			{"app.kubernetes.io/name", app},
		}
	} else {
		selectors = [][]string{
			{"app.kubernetes.io/component", "staging"},
			{"app.kubernetes.io/managed-by", "epinio"},
			{models.EpinioStageIDLabel, stageID},
			{"app.kubernetes.io/part-of", org},
		}
	}

	for _, req := range selectors {
		req, err := labels.NewRequirement(req[0], selection.Equals, []string{req[1]})
		if err != nil {
			return err
		}
		selector = selector.Add(*req)
	}

	config := &tailer.Config{
		ContainerQuery:        regexp.MustCompile(".*"),
		ExcludeContainerQuery: regexp.MustCompile("linkerd-(proxy|init)"),
		ContainerState:        "running",
		Exclude:               nil,
		Include:               nil,
		Timestamps:            false,
		Since:                 duration.LogHistory(),
		AllNamespaces:         true,
		LabelSelector:         selector,
		TailLines:             nil,
		Namespace:             "",
		PodQuery:              regexp.MustCompile(".*"),
	}

	if follow {
		return tailer.StreamLogs(ctx, logChan, wg, config, cluster)
	}

	return tailer.FetchLogs(ctx, logChan, wg, config, cluster)
}
