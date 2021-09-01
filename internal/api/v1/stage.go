package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/spf13/viper"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tekton "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/auth"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/organizations"
)

const (
	LocalRegistry = "127.0.0.1:30500/apps"
)

type stageParam struct {
	models.AppRef
	Git          *models.GitRef
	Stage        models.StageRef
	Owner        metav1.OwnerReference
	Environment  models.EnvVariableList
	RegistryURL  string
	BuilderImage string
	Username     string
}

// GitURL returns the gitea repository URL by combining the server
// with the org and name
func (app *stageParam) GitURL(server string) string {
	return fmt.Sprintf("%s/%s/%s", server, app.Org, app.Name)
}

// ImageURL returns the URL of the docker image to be, using the
// ImageID. The ImageURL is later used in app.yml and to send in the
// stage response.
func (app *stageParam) ImageURL(registryURL string) string {
	return fmt.Sprintf("%s/%s-%s", registryURL, app.Name, app.Git.Revision)
}

// ensurePVC is a helper creating the kube PVC associated with an
// application, if needed, i.e. not already present.
func (c ApplicationsController) ensurePVC(ctx context.Context, cluster *kubernetes.Cluster, pvcName string) error {
	_, err := cluster.Kubectl.CoreV1().PersistentVolumeClaims(deployments.TektonStagingNamespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) { // Unknown error, irrelevant to non-existence
		return err
	}
	if err == nil { // pvc already exists
		return nil
	}

	// From here on, only if the PVC is missing
	_, err = cluster.Kubectl.CoreV1().PersistentVolumeClaims(deployments.TektonStagingNamespace).
		Create(ctx, &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pvcName,
				Namespace: deployments.TektonStagingNamespace,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.ResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
		}, metav1.CreateOptions{})

	return err
}

// Stage handles the API endpoint /orgs/:org/applications/:app/stage
// It creates a Tekton PipelineRun resource to stage the app
func (hc ApplicationsController) Stage(w http.ResponseWriter, r *http.Request) APIErrors {
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

	req := models.StageRequest{}
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		return NewBadRequest("Failed to construct an Application from the request", err.Error())
	}

	if name != req.App.Name {
		return NewBadRequest("name parameter from URL does not match name param in body")
	}
	if org != req.App.Org {
		return NewBadRequest("org parameter from URL does not match org param in body")
	}

	if req.BuilderImage == "" {
		return NewBadRequest("builder image cannot be empty")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err, "failed to get access to a kube client")
	}

	// check application resource
	app, err := application.Get(ctx, cluster, req.App)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return AppIsNotKnown("cannot stage app, application resource is missing")
		}
		return InternalError(err, "failed to get the application resource")
	}

	log.Info("staging app", "org", org, "app", req)

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

	environment, err := application.Environment(ctx, cluster, req.App)
	if err != nil {
		return InternalError(err, "failed to access application runtime environment")
	}

	owner := metav1.OwnerReference{
		APIVersion: app.GetAPIVersion(),
		Kind:       app.GetKind(),
		Name:       app.GetName(),
		UID:        app.GetUID(),
	}

	mainDomain, err := domain.MainDomain(ctx)
	if err != nil {
		return InternalError(err)
	}
	params := stageParam{
		Username:     username,
		AppRef:       req.App,
		Git:          req.Git,
		Owner:        owner,
		Environment:  environment,
		RegistryURL:  fmt.Sprintf("%s.%s/%s", deployments.RegistryDeploymentID, mainDomain, "apps"),
		BuilderImage: req.BuilderImage,
	}

	err = hc.ensurePVC(ctx, cluster, req.App.PVCName())
	if err != nil {
		return InternalError(err, "failed to ensure a PersistenVolumeClaim for the application source and cache")
	}

	pr := newPipelineRun(uid, params)
	o, err := client.Create(ctx, pr, metav1.CreateOptions{})
	if err != nil {
		return InternalError(err, fmt.Sprintf("failed to create pipeline run: %#v", o))
	}

	cert := auth.CertParam{
		Name:      params.Name,
		Namespace: params.Org,
		Issuer:    viper.GetString("tls-issuer"),
		Domain:    mainDomain,
	}

	log.Info("app cert", "domain", cert.Domain, "issuer", cert.Issuer)

	err = auth.CreateCertificate(ctx, cluster, cert, &owner)
	if err != nil {
		return InternalError(err)
	}

	log.Info("staged app", "org", org, "app", params.AppRef, "uid", uid)
	// The ImageURL in the response should be the one accessible by kubernetes.
	// In stageParam above, the registry is passed with the registry ingress url,
	// since it's where tekton will push.
	if viper.GetBool("use-internal-registry-node-port") {
		params.RegistryURL = LocalRegistry
	}
	resp := models.StageResponse{
		Stage:    models.NewStage(uid),
		ImageURL: params.ImageURL(params.RegistryURL),
	}
	err = jsonResponse(w, resp)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// Staged handles the API endpoint /orgs/:org/staging/:stage_id/complete
// It waits for the Tekton PipelineRun resource staging the app to complete
func (hc ApplicationsController) Staged(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()

	p := httprouter.ParamsFromContext(ctx)
	org := p.ByName("org")
	id := p.ByName("stage_id")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return InternalError(err)
	}

	cs, err := tekton.NewForConfig(cluster.RestConfig)
	if err != nil {
		return InternalError(err)
	}

	client := cs.TektonV1beta1().PipelineRuns(deployments.TektonStagingNamespace)

	err = wait.PollImmediate(time.Second, duration.ToAppBuilt(),
		func() (bool, error) {
			l, err := client.List(ctx, metav1.ListOptions{LabelSelector: models.EpinioStageIDLabel + "=" + id})
			if err != nil {
				return false, err
			}
			if len(l.Items) == 0 {
				return false, nil
			}
			for _, pr := range l.Items {
				// any failed conditions, throw an error so we can exit early
				for _, c := range pr.Status.Conditions {
					if c.IsFalse() {
						return false, errors.New(c.Message)
					}
				}
				// it worked
				if pr.Status.CompletionTime != nil {
					return true, nil
				}
			}
			// pr exists, but still running
			return false, nil
		})

	if err != nil {
		return InternalError(err)
	}

	return nil
}

// newPipelineRun is a helper which creates a Tekton pipeline run
// resource from the given staging id and app name
func newPipelineRun(uid string, app stageParam) *v1beta1.PipelineRun {
	str := v1beta1.NewArrayOrString

	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: uid,
			Labels: map[string]string{
				"app.kubernetes.io/name":       app.Name,
				"app.kubernetes.io/part-of":    app.Org,
				"app.kubernetes.io/username":   app.Username,
				models.EpinioStageIDLabel:      uid,
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/component":  "staging",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			ServiceAccountName: "staging-triggers-admin",
			PipelineRef:        &v1beta1.PipelineRef{Name: "staging-pipeline"},
			Params: []v1beta1.Param{
				{Name: "APP_NAME", Value: *str(app.Name)},
				{Name: "ORG", Value: *str(app.Org)},
				{Name: "APP_IMAGE", Value: *str(app.ImageURL(app.RegistryURL))},
				{Name: "STAGE_ID", Value: *str(uid)},
				{Name: "BUILDER_IMAGE", Value: *str(app.BuilderImage)},
				{Name: "ENV_VARS", Value: v1beta1.ArrayOrString{
					Type:     v1beta1.ParamTypeArray,
					ArrayVal: app.Environment.StagingEnvArray()},
				},
			},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name:    "cache",
					SubPath: "cache",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: app.PVCName(),
						ReadOnly:  false,
					},
				},
				{
					Name:    "source",
					SubPath: "source",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: app.PVCName(),
						ReadOnly:  false,
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
							{Name: "url", Value: app.Git.URL},
						},
					},
				},
			},
		},
	}
}
