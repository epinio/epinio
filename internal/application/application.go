// Package application collects the structures and functions that deal
// with application workloads on k8s
package application

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/s3manager"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	appv1beta1 "sigs.k8s.io/application/api/v1beta1"
)

// Create generates a new kube app resource in the namespace of the
// organization. Note that this is the passive resource holding the
// app's configuration. It is not the active workload
func Create(ctx context.Context, cluster *kubernetes.Cluster, app models.AppRef, username string) error {
	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	// we create the appCRD in the org's namespace
	obj := &appv1beta1.Application{
		Spec: appv1beta1.ApplicationSpec{
			Descriptor: appv1beta1.Descriptor{
				Type: "epinio-workload",
				Maintainers: []appv1beta1.ContactData{
					{
						Name: username,
					},
				},
				Owners: []appv1beta1.ContactData{
					{
						Name: username,
					},
				},
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

// Get returns the application resource from the cluster.  This should be
// changed to return a typed application struct, like appv1beta1.Application if
// needed in the future.
func Get(ctx context.Context, cluster *kubernetes.Cluster, app models.AppRef) (*unstructured.Unstructured, error) {
	client, err := cluster.ClientApp()
	if err != nil {
		return nil, err
	}

	return client.Namespace(app.Org).Get(ctx, app.Name, metav1.GetOptions{})
}

// Exists checks if the named application exists or not, and returns an appropriate boolean flag
func Exists(ctx context.Context, cluster *kubernetes.Cluster, app models.AppRef) (bool, error) {
	_, err := Get(ctx, cluster, app)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ListAppRefs returns an app reference for every application resource in the org's namespace
func ListAppRefs(ctx context.Context, cluster *kubernetes.Cluster, org string) ([]models.AppRef, error) {
	client, err := cluster.ClientApp()
	if err != nil {
		return nil, err
	}

	list, err := client.Namespace(org).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	apps := make([]models.AppRef, 0, len(list.Items))
	for _, app := range list.Items {
		apps = append(apps, models.NewAppRef(app.GetName(), org))
	}

	return apps, nil
}

// Lookup locates the workload of the named application (and org). The
// result is nil if the application exists and is not active.
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

// List returns a list of all available workloads (in the org)
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
		return result, fmt.Errorf("namespace %s does not exist", org)
	}

	deployments, err := cluster.Kubectl.AppsV1().Deployments(org).List(ctx, listOptions)
	if err != nil {
		return result, err
	}

	for _, deployment := range deployments.Items {
		w := NewWorkload(cluster, models.NewAppRef(deployment.ObjectMeta.Name, org))
		appEpinio, err := w.Complete(ctx)
		if err != nil {
			return result, err
		}

		result = append(result, *appEpinio)
	}

	return result, nil
}

// ListApps returns a list of all available apps (in the org)
func ListApps(ctx context.Context, cluster *kubernetes.Cluster, org string) (models.AppList, error) {
	result := models.AppList{}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return result, err
	}
	if !exists {
		return result, fmt.Errorf("namespace %s does not exist", org)
	}

	// Get references for all apps, deployed or not

	appRefs, err := ListAppRefs(ctx, cluster, org)
	if err != nil {
		return result, err
	}

	// Get apps with workloads

	apps, err := List(ctx, cluster, org)
	if err != nil {
		return result, err
	}

	// Fuse the two, to get a list of all apps. The undeployed apps have partially filled
	// structure. The fields related to deployment are left unfilled.  To fuse the deployed apps
	// are mapped for quick access by name, and then an iteration over the refs assembles the
	// final output, taking either a deployed app, or creating a partial filled.

	appMap := make(map[string]models.App)
	for _, app := range apps {
		appMap[app.Name] = app
	}

	for _, ref := range appRefs {
		app, ok := appMap[ref.Name]
		if !ok {
			app = *models.NewApp(ref.Name, ref.Org)
			app.Status = `Inactive, without workload. Launch via "epinio app push"`
		}

		result = append(result, app)
	}

	return result, nil
}

// Delete removes the named application, its workload (if active), bindings (if any),
// the stored application sources, and any pipelineruns from when the application was
// staged (if active). Waits for the application's deployment's pods to disappear
// (if active).
func Delete(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) error {
	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	// get application workload, if any, and its bounded services, if any
	app, err := Lookup(ctx, cluster, appRef.Org, appRef.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// delete all service bindings
	if app != nil {
		workload := NewWorkload(cluster, appRef)
		err = workload.UnbindAll(ctx, cluster, app.BoundServices)
		if err != nil {
			return err
		}
	}

	// delete application resource, will cascade and delete deployment,
	// ingress, service and certificate
	err = client.Namespace(appRef.Org).Delete(ctx, appRef.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	// delete pipelineruns in tekton-staging namespace
	err = Unstage(ctx, cluster, appRef, "")
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// delete staging PVC (the one that stores "source" and "cache" tekton workspaces)
	err = DeleteStagePVC(ctx, cluster, appRef)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	err = cluster.WaitForPodBySelectorMissing(ctx, nil,
		appRef.Org,
		fmt.Sprintf("app.kubernetes.io/name=%s", appRef.Name),
		duration.ToDeployment())
	if err != nil {
		return err
	}

	return nil
}

// DeleteStagePVC removes the kube PVC resource which was used to hold the application sources for Tekton, during staging.
func DeleteStagePVC(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) error {
	return cluster.Kubectl.CoreV1().
		PersistentVolumeClaims(deployments.TektonStagingNamespace).Delete(ctx, appRef.PVCName(), metav1.DeleteOptions{})
}

// Unstage deletes either all PipelineRuns of the named application, or all but the current.
// It also deletes the relevant (old) objects from the S3 storage.
func Unstage(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, stageIDCurrent string) error {
	cs, err := versioned.NewForConfig(cluster.RestConfig)
	if err != nil {
		return err
	}

	s3ConnectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster,
		deployments.TektonStagingNamespace, deployments.S3ConnectionDetailsSecret)
	if err != nil {
		return errors.Wrap(err, "fetching the S3 connection details from the Kubernetes secret")
	}
	s3m, err := s3manager.New(s3ConnectionDetails)
	if err != nil {
		return errors.Wrap(err, "creating an S3 manager")
	}

	client := cs.TektonV1beta1().PipelineRuns(deployments.TektonStagingNamespace)

	l, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s",
			appRef.Name, appRef.Org),
	})
	if err != nil {
		return err
	}

	for _, pr := range l.Items {
		if stageIDCurrent != "" && stageIDCurrent == pr.ObjectMeta.Name {
			continue
		}

		err := client.Delete(ctx, pr.ObjectMeta.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}

		if err = s3m.DeleteObject(ctx, pr.ObjectMeta.Labels[models.EpinioStageBlobUIDLabel]); err != nil {
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
	logger := tracelog.NewLogger().WithName("logs-backend").V(2)
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
		logger.Info("stream")
		return tailer.StreamLogs(ctx, logChan, wg, config, cluster)
	}

	logger.Info("fetch")
	return tailer.FetchLogs(ctx, logChan, wg, config, cluster)
}
