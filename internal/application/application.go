// Package application has actor functions that deal with application workloads
// on k8s
package application

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/organizations"
	pkgerrors "github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	appv1beta1 "sigs.k8s.io/application/api/v1beta1"
)

type GiteaInterface interface {
	DeleteRepo(org, repo string) error
}

func Create(ctx context.Context, cluster *kubernetes.Cluster, app models.AppRef) error {
	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	// we create the appCRD in the org's namespace
	obj := &appv1beta1.Application{
		Spec: appv1beta1.ApplicationSpec{
			Descriptor: appv1beta1.Descriptor{
				Type: "epinio-workload",
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

// Lookup locates a workload by org and name
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

// List returns an list of all available workloads (in the org)
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
		w := NewWorkload(cluster, models.NewAppRef(deployment.ObjectMeta.Name, org))
		appEpinio, err := w.Complete(ctx)
		if err != nil {
			return result, err
		}

		result = append(result, *appEpinio)
	}

	return result, nil
}

// Delete an application, optionally its workload, bindings and git repo.
// Finally unstage its pipelineruns and wait for the deployment's pods to disappear.
func Delete(ctx context.Context, cluster *kubernetes.Cluster, gitea GiteaInterface, appRef models.AppRef, app *models.App) error {
	if app != nil {
		err := NewWorkload(cluster, appRef).UnbindAll(ctx, cluster, app.BoundServices)
		if err != nil {
			return err
		}

		// if there was a workload, there should also be a gitea repo
		err = gitea.DeleteRepo(appRef.Org, appRef.Name)
		if err != nil {
			return pkgerrors.Wrap(err, "failed to delete repository")
		}
	}

	// delete application resource, will cascade and delete deployment,
	// ingress, service and certificate
	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	err = client.Namespace(appRef.Org).Delete(ctx, appRef.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	// delete pipelineruns in tekton-staging namespace
	err = Unstage(ctx, appRef.Name, appRef.Org, "")
	if err != nil {
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
