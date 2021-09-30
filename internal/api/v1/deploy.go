package v1

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/julienschmidt/httprouter"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

const (
	DefaultInstances = int32(1)
)

type deployParam struct {
	models.AppRef
	helmChartURL string
	imageURL     string
	username     string
	replicaCount int32
	stage        models.StageRef
	ownerUID     string
	route        string

	environment models.EnvVariableList
	services    application.AppServiceBindList
}

// Deploy handles the API endpoint /orgs/:org/applications/:app/deploy
// It creates the deployment, service and ingress (kube) resources for the app
func (hc ApplicationsController) Deploy(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	log := tracelog.Logger(ctx)

	p := httprouter.ParamsFromContext(ctx)
	org := p.ByName("org")
	name := p.ByName("app")
	username, err := GetUsername(r)
	if err != nil {
		return UserNotFound()
	}

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	req := models.DeployRequest{}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		return NewBadRequest("Failed to unmarshal deploy request ", err.Error())
	}

	if name != req.App.Name {
		return NewBadRequest("name parameter from URL does not match name param in body")
	}
	if org != req.App.Org {
		return NewBadRequest("org parameter from URL does not match org param in body")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err, "failed to get access to a kube client")
	}

	// check application resource
	applicationCR, err := application.Get(ctx, cluster, req.App)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return AppIsNotKnown("cannot deploy app, application resource is missing")
		}
		return InternalError(err, "failed to get the application resource")
	}
	owner := metav1.OwnerReference{
		APIVersion: applicationCR.GetAPIVersion(),
		Kind:       applicationCR.GetKind(),
		Name:       applicationCR.GetName(),
		UID:        applicationCR.GetUID(),
	}

	// determine number of desired instances
	instances, err := application.Scaling(ctx, cluster, req.App)
	if err != nil {
		return InternalError(err, "failed to access application's desired instances")
	}

	// determine runtime environment, if any
	environment, err := application.Environment(ctx, cluster, req.App)
	if err != nil {
		return InternalError(err, "failed to access application's runtime environment")
	}

	// determine bound services, if any
	services, err := application.BoundServices(ctx, cluster, req.App)
	if err != nil {
		return InternalError(err, "failed to access application's bound services")
	}

	bindings, err := application.ToBinds(ctx, services, req.App.Name, username)
	if err != nil {
		return InternalError(err, "failed to process application's bound services")
	}

	route, err := domain.AppDefaultRoute(ctx, req.App.Name)
	if err != nil {
		return InternalError(err)
	}

	params := deployParam{
		AppRef:   req.App,
		imageURL: req.ImageURL,
		stage:    req.Stage,

		username:     username,
		replicaCount: instances,
		route:        route,

		helmChartURL: "https://github.com/manno/epinio-helm-app/raw/main/repo/epinio-app-0.1.0.tgz",

		// TODO
		environment: environment,
		services:    bindings,
		//ownerUID:     owner.UID,
	}

	log.Info("deploying app via helm", "org", org, "app", req.App)

	client, err := cluster.ClientTekton()
	if err != nil {
		return InternalError(err, "failed to get access to a tekton client")
	}

	// TODO uninstall first if this is an update
	pr := newDeployPR(params)
	o, err := client.PipelineRuns(deployments.TektonStagingNamespace).Create(ctx, pr, metav1.CreateOptions{})
	if err != nil {
		return InternalError(err, fmt.Sprintf("failed to create pipeline run: %#v", o))
	}

	// Delete previous pipelineruns except for the current one
	if req.Stage.ID != "" {
		if err := application.Unstage(ctx, cluster, req.App, req.Stage.ID); err != nil {
			return InternalError(err)
		}
	}

	resp := models.DeployResponse{
		Route: route,
	}
	err = jsonResponse(w, resp)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func newDeployPR(p deployParam) *v1beta1.PipelineRun {
	str := v1beta1.NewArrayOrString

	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deploy-" + p.stage.ID,
			Labels: map[string]string{
				"app.kubernetes.io/name":       p.Name,
				"app.kubernetes.io/part-of":    p.Org,
				"app.kubernetes.io/created-by": p.username,
				models.EpinioStageIDLabel:      p.stage.ID,
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/component":  "deploy",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			ServiceAccountName: "staging-triggers-admin",
			PipelineRef:        &v1beta1.PipelineRef{Name: "deploy-pipeline"},
			Params: []v1beta1.Param{
				{Name: "APP_NAME", Value: *str(p.Name)},
				{Name: "NAMESPACE", Value: *str(p.Org)},
				{Name: "IMAGE_URL", Value: *str(p.imageURL)},
				{Name: "STAGE_ID", Value: *str(p.stage.ID)},
				{Name: "USERNAME", Value: *str(p.username)},
				{Name: "REPLICA_COUNT", Value: *str(strconv.Itoa(int(p.replicaCount)))},
				{Name: "ROUTE", Value: *str(p.route)},
				{Name: "HELM_CHART_URL", Value: *str(p.helmChartURL)},
			},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name:     "helm",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},
	}
}
