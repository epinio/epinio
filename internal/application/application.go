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

	epinioerrors "github.com/epinio/epinio/internal/errors"
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

// CurrentlyStaging returns true if there is an active (not completed) PipelineRun
// for this application.
func CurrentlyStaging(ctx context.Context, cluster *kubernetes.Cluster, namespace, appName string) (bool, error) {
	tc, err := cluster.ClientTekton()
	if err != nil {
		return false, err
	}
	client := tc.PipelineRuns(deployments.TektonStagingNamespace)
	l, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s", appName, namespace),
	})
	if err != nil {
		return false, err
	}

	// assume that completed pipelineruns are from the past and have a CompletionTime
	for _, pr := range l.Items {
		if pr.Status.CompletionTime == nil {
			return true, nil
		}
	}

	return false, nil
}

// Lookup locates the named application (and org).
func Lookup(ctx context.Context, cluster *kubernetes.Cluster, org, appName string) (*models.App, error) {
	meta := models.NewAppRef(appName, org)

	ok, err := Exists(ctx, cluster, meta)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	app := meta.App()

	err = fetch(ctx, cluster, app)
	if err != nil {
		app.StatusMessage = err.Error()
		app.Status = models.ApplicationError
	} else {
		err = calculateStatus(ctx, cluster, app)
		if err != nil {
			return app, err
		}
	}

	return app, nil
}

// ListAppRefs returns an app reference for every application resource in the specified
// namespace. If no namespace is specified (empty string) then apps across all namespaces are
// returned.
func ListAppRefs(ctx context.Context, cluster *kubernetes.Cluster, namespace string) ([]models.AppRef, error) {
	client, err := cluster.ClientApp()
	if err != nil {
		return nil, err
	}

	list, err := client.Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	apps := make([]models.AppRef, 0, len(list.Items))
	for _, app := range list.Items {
		apps = append(apps, models.NewAppRef(app.GetName(), app.GetNamespace()))
	}

	return apps, nil
}

// List returns a list of all available apps in the specified namespace. If no namespace is
// specified (empty string) then apps across all namespaces are returned.
func List(ctx context.Context, cluster *kubernetes.Cluster, namespace string) (models.AppList, error) {

	// Verify namespace, if specified

	if namespace != "" {
		exists, err := organizations.Exists(ctx, cluster, namespace)
		if err != nil {
			return models.AppList{}, err
		}
		if !exists {
			return models.AppList{}, epinioerrors.NamespaceMissingError{Namespace: namespace}
		}
	}

	// Get references for all apps, deployed or not

	appRefs, err := ListAppRefs(ctx, cluster, namespace)
	if err != nil {
		return models.AppList{}, err
	}

	// Convert references to full application structures

	result := models.AppList{}

	for _, ref := range appRefs {
		app, err := Lookup(ctx, cluster, ref.Org, ref.Name)
		if err != nil {
			return result, err
		}
		result = append(result, *app)
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

	// delete application resource, will cascade and delete deployment,
	// ingress, service and certificate, environment variables, bindings
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
	err = deleteStagePVC(ctx, cluster, appRef)
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

// deleteStagePVC removes the kube PVC resource which was used to hold the application sources for Tekton, during staging.
func deleteStagePVC(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) error {
	return cluster.Kubectl.CoreV1().
		PersistentVolumeClaims(deployments.TektonStagingNamespace).Delete(ctx, appRef.MakePVCName(), metav1.DeleteOptions{})
}

// Unstage removes staging resources. It deletes either all PipelineRuns of the
// named application, or all but stageIDCurrent. It also deletes the staged
// objects from the S3 storage.
func Unstage(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, stageIDCurrent string) error {
	s3ConnectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster,
		deployments.TektonStagingNamespace, deployments.S3ConnectionDetailsSecret)
	if err != nil {
		return errors.Wrap(err, "fetching the S3 connection details from the Kubernetes secret")
	}
	s3m, err := s3manager.New(s3ConnectionDetails)
	if err != nil {
		return errors.Wrap(err, "creating an S3 manager")
	}

	tc, err := cluster.ClientTekton()
	if err != nil {
		return err
	}

	client := tc.PipelineRuns(deployments.TektonStagingNamespace)

	l, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s",
			appRef.Name, appRef.Org),
	})
	if err != nil {
		return err
	}

	for _, pr := range l.Items {
		id := pr.Labels[models.EpinioStageIDLabel]
		// stageIDCurrent is either empty or the id to keep
		if stageIDCurrent != "" && stageIDCurrent == id {
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

// fetch is a common helper for Lookup and List. It fetches all
// information about an application from the cluster.
func fetch(ctx context.Context, cluster *kubernetes.Cluster, app *models.App) error {
	// Consider delayed loading, i.e. on first access, or for transfer (API response).
	// Consider objects for the information which hide the defered loading.
	// These could also have the necessary modifier methods.
	// See sibling files scale.go, env.go, services.go.
	// Defered at the moment, the PR is big enough already.

	environment, err := Environment(ctx, cluster, app.Meta)
	if err != nil {
		return err
	}

	instances, err := Scaling(ctx, cluster, app.Meta)
	if err != nil {
		return err
	}

	services, err := BoundServiceNames(ctx, cluster, app.Meta)
	if err != nil {
		return err
	}

	app.Configuration.Instances = &instances
	app.Configuration.Services = services
	app.Configuration.Environment = environment

	// Check if app is active, and if yes, fill the associated parts.
	// May have to straighten the workload structure a bit further.

	wl := NewWorkload(cluster, app.Meta)

	deployment, err := wl.Deployment(ctx)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// App is inactive, no deployment, no workload
		return nil
	}

	app.Workload = wl.Get(ctx, deployment)

	return nil
}

// calculateStatus sets the Status field of the App object.
// To decide what the status value should be, it combines various pieces of information.
//- If Status is ApplicationError, leave it as it (it was set by "Lookup")
//- If there is a pipelinerun, app is: ApplicationStaging
//- If there is no workload and no pipeline run, app is: ApplicationCreated
//- If there is no pipelinerun and there is a workload, app is: ApplicationRunning
func calculateStatus(ctx context.Context, cluster *kubernetes.Cluster, app *models.App) error {
	if app.Status == models.ApplicationError {
		return nil
	}
	staging, err := CurrentlyStaging(ctx, cluster, app.Meta.Org, app.Meta.Name)
	if err != nil {
		return err
	}
	if staging {
		app.Status = models.ApplicationStaging
		return nil
	}
	if app.Workload == nil {
		app.Status = models.ApplicationCreated
		return nil
	}

	app.Status = models.ApplicationRunning

	return nil
}
