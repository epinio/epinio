package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/epinio/epinio/internal/domain"
	"github.com/julienschmidt/httprouter"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
)

const (
	RegistryURL      = "registry.epinio-registry/apps"
	DefaultInstances = int32(1)
)

type stageParam struct {
	models.AppRef
	Image     models.ImageRef
	Git       *models.GitRef
	Route     string
	Stage     models.StageRef
	Instances int32
}

// GitURL returns the git URL by combining the server with the org and name
func (app *stageParam) GitURL(server string) string {
	return fmt.Sprintf("%s/%s/%s", server, app.Org, app.Name)
}

// ImageURL returns the URL of the image, using the ImageID. The ImageURL is
// later used in app.yml.  Since the final commit is not known when the app.yml
// is written, we cannot use Repo.Revision
func (app *stageParam) ImageURL(server string) string {
	return fmt.Sprintf("%s/%s-%s", server, app.Name, app.Git.Revision)
}

// Stage will create a Tekton PipelineRun resource to stage and start the app
func (hc ApplicationsController) Stage(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	name := params.ByName("app")

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	req := models.StageRequest{}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		return NewBadRequest("Failed to construct an Application from the request", err.Error())
	}

	if name != req.App.Name {
		return NewBadRequest("name parameter from URL does not match name param in body")
	}

	if req.Instances != nil && *req.Instances < 0 {
		return NewBadRequest("instances param should be integer equal or greater than zero")
	}

	log.Info("staging app", "org", org, "app", req)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err, "failed to get access to a kube client")
	}

	cs, err := versioned.NewForConfig(cluster.RestConfig)
	if err != nil {
		return InternalError(err, "failed to get access to a tekton client")
	}
	client := cs.TektonV1beta1().PipelineRuns(deployments.TektonStagingNamespace)

	uid, err := randstr.Hex16()
	if err != nil {
		return InternalError(err, "failed to generate a uid")
	}

	l, err := client.List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s", req.App.Name, req.App.Org),
	})
	if err != nil {
		return InternalError(err)
	}

	// assume that completed pipelineruns are from the past and have a CompletionTime
	for _, pr := range l.Items {
		if pr.Status.CompletionTime == nil {
			return NewBadRequest("pipelinerun for image ID still running")
		}
	}

	// find out the instances
	var instances int32
	if req.Instances != nil {
		instances = int32(*req.Instances)
	} else {
		instances, err = existingReplica(ctx, cluster.Kubectl, req.App)
		if err != nil {
			return InternalError(err)
		}
	}

	app := stageParam{
		AppRef:    req.App,
		Git:       req.Git,
		Route:     req.Route,
		Instances: instances,
	}

	pr := newPipelineRun(uid, app)
	o, err := client.Create(ctx, pr, metav1.CreateOptions{})
	if err != nil {
		return InternalError(err, fmt.Sprintf("failed to create pipeline run: %#v", o))
	}

	mainDomain, err := domain.MainDomain(ctx)
	if err != nil {
		return InternalError(err)
	}

	err = auth.CreateCertificate(ctx, cluster.RestConfig, app.Name, app.Org, mainDomain)
	if err != nil {
		return InternalError(err)
	}

	log.Info("staged app", "org", org, "app", app.AppRef, "uid", uid)

	resp := models.StageResponse{Stage: models.NewStage(uid)}
	err = jsonResponse(w, resp)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func existingReplica(ctx context.Context, client *k8s.Clientset, app models.AppRef) (int32, error) {
	// if a deployment exists, use that deployment's replica count
	result, err := client.AppsV1().Deployments(app.Org).Get(ctx, app.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return DefaultInstances, nil
		}
		return 0, err
	}
	return *result.Spec.Replicas, nil
}

func newPipelineRun(uid string, app stageParam) *v1beta1.PipelineRun {
	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: uid,
			Labels: map[string]string{
				"app.kubernetes.io/name":       app.Name,
				"app.kubernetes.io/part-of":    app.Org,
				models.EpinioStageIDLabel:      uid,
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/component":  "staging",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			ServiceAccountName: "staging-triggers-admin",
			PipelineRef:        &v1beta1.PipelineRef{Name: "staging-pipeline"},
			Params: []v1beta1.Param{
				{Name: "APP_NAME", Value: *v1beta1.NewArrayOrString(app.Name)},
				{Name: "ORG", Value: *v1beta1.NewArrayOrString(app.Org)},
				{Name: "ROUTE", Value: *v1beta1.NewArrayOrString(app.Route)},
				{Name: "INSTANCES", Value: *v1beta1.NewArrayOrString(strconv.Itoa(int(app.Instances)))},
				{Name: "APP_IMAGE", Value: *v1beta1.NewArrayOrString(app.ImageURL(RegistryURL))},
				{Name: "DEPLOYMENT_IMAGE", Value: *v1beta1.NewArrayOrString(app.ImageURL(gitea.LocalRegistry))},
				{Name: "STAGE_ID", Value: *v1beta1.NewArrayOrString(uid)},
			},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name: "source",
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
								corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("1Gi"),
							}},
						},
					},
				},
			},
			Resources: []v1beta1.PipelineResourceBinding{
				{
					Name: "source-repo",
					ResourceSpec: &v1alpha1.PipelineResourceSpec{
						Type: v1alpha1.PipelineResourceTypeGit,
						Params: []v1alpha1.ResourceParam{
							{Name: "revision", Value: app.Git.Revision},
							{Name: "url", Value: app.GitURL(deployments.GiteaURL)},
						},
					},
				},
			},
		},
	}
}
