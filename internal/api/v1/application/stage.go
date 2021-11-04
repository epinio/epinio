package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
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
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/s3manager"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

type stageParam struct {
	models.AppRef
	BlobUID             string
	BuilderImage        string
	Environment         models.EnvVariableList
	Owner               metav1.OwnerReference
	RegistryURL         string
	S3ConnectionDetails s3manager.ConnectionDetails
	Stage               models.StageRef
	Username            string
}

// ImageURL returns the URL of the container image to be, using the
// ImageID. The ImageURL is later used in app.yml and to send in the
// stage response.
func (app *stageParam) ImageURL(registryURL string) string {
	return fmt.Sprintf("%s/%s-%s", registryURL, app.Name, app.Stage.ID)
}

// ensurePVC creates a PVC for the application if one doesn't already exist.
// This PVC is used to store the application source blobs (as they are uploaded
// on the "upload" endpoint). It's also mounted in the staging task pod as the
// "source" tekton workspace.
// The same PVC stores the application's build cache (on a separate directory).
func ensurePVC(ctx context.Context, cluster *kubernetes.Cluster, ar models.AppRef) error {
	_, err := cluster.Kubectl.CoreV1().PersistentVolumeClaims(deployments.TektonStagingNamespace).
		Get(ctx, ar.MakePVCName(), metav1.GetOptions{})
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
				Name:      ar.MakePVCName(),
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
func (hc Controller) Stage(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := tracelog.Logger(ctx)

	org := c.Param("org")
	name := c.Param("app")
	username := requestctx.User(ctx)

	req := models.StageRequest{}
	if err := c.BindJSON(&req); err != nil {
		return apierror.NewBadRequest("Failed to unmarshal app stage request", err.Error())
	}

	if name != req.App.Name {
		return apierror.NewBadRequest("name parameter from URL does not match name param in body")
	}
	if org != req.App.Org {
		return apierror.NewBadRequest("org parameter from URL does not match org param in body")
	}

	if req.BuilderImage == "" {
		return apierror.NewBadRequest("builder image cannot be empty")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}

	// check application resource
	app, err := application.Get(ctx, cluster, req.App)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierror.AppIsNotKnown("cannot stage app, application resource is missing")
		}
		return apierror.InternalError(err, "failed to get the application resource")
	}

	log.Info("staging app", "org", org, "app", req)

	staging, err := application.CurrentlyStaging(ctx, cluster, req.App.Org, req.App.Name)
	if err != nil {
		return apierror.InternalError(err)
	}
	if staging {
		return apierror.NewBadRequest("pipelinerun for image ID still running")
	}

	uid, err := randstr.Hex16()
	if err != nil {
		return apierror.InternalError(err, "failed to generate a uid")
	}

	environment, err := application.Environment(ctx, cluster, req.App)
	if err != nil {
		return apierror.InternalError(err, "failed to access application runtime environment")
	}

	owner := metav1.OwnerReference{
		APIVersion: app.GetAPIVersion(),
		Kind:       app.GetKind(),
		Name:       app.GetName(),
		UID:        app.GetUID(),
	}

	s3ConnectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster, deployments.TektonStagingNamespace, deployments.S3ConnectionDetailsSecret)
	if err != nil {
		return apierror.InternalError(err, "failed to fetch the S3 connection details")
	}

	registryPublicURL, err := domain.EpinioRegistryPublicURL(ctx)
	if err != nil {
		return apierror.InternalError(err, "getting the Epinio registry public URL")
	}
	params := stageParam{
		AppRef:              req.App,
		BuilderImage:        req.BuilderImage,
		BlobUID:             req.BlobUID,
		Environment:         environment.List(),
		Owner:               owner,
		RegistryURL:         registryPublicURL,
		S3ConnectionDetails: s3ConnectionDetails,
		Stage:               models.NewStage(uid),
		Username:            username,
	}

	err = ensurePVC(ctx, cluster, req.App)
	if err != nil {
		return apierror.InternalError(err, "failed to ensure a PersistenVolumeClaim for the application source and cache")
	}

	tc, err := cluster.ClientTekton()
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a tekton client")
	}
	client := tc.PipelineRuns(deployments.TektonStagingNamespace)
	pr := newPipelineRun(params)
	o, err := client.Create(ctx, pr, metav1.CreateOptions{})
	if err != nil {
		return apierror.InternalError(err, fmt.Sprintf("failed to create pipeline run: %#v", o))
	}

	log.Info("staged app", "org", org, "app", params.AppRef, "uid", uid)

	response.OKReturn(c, models.StageResponse{
		Stage:    models.NewStage(uid),
		ImageURL: params.ImageURL(params.RegistryURL),
	})
	return nil
}

// Staged handles the API endpoint /orgs/:org/staging/:stage_id/complete
// It waits for the Tekton PipelineRun resource staging the app to complete
func (hc Controller) Staged(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	org := c.Param("org")
	id := c.Param("stage_id")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return apierror.InternalError(err)
	}

	if !exists {
		return apierror.InternalError(err)
	}

	cs, err := tekton.NewForConfig(cluster.RestConfig)
	if err != nil {
		return apierror.InternalError(err)
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
		return apierror.InternalError(err)
	}

	response.OK(c)
	return nil
}

// newPipelineRun is a helper which creates a Tekton pipeline run
// resource from the given staging params
func newPipelineRun(app stageParam) *v1beta1.PipelineRun {
	str := v1beta1.NewArrayOrString

	protocol := "http"
	if app.S3ConnectionDetails.UseSSL {
		protocol = "https"
	}
	awsScript := fmt.Sprintf("aws --endpoint-url %s://$1 s3 cp s3://$2/$3 $(workspaces.source.path)/$3", protocol)
	awsArgs := []string{
		app.S3ConnectionDetails.Endpoint,
		app.S3ConnectionDetails.Bucket,
		app.BlobUID,
	}

	return &v1beta1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: app.Stage.ID,
			Labels: map[string]string{
				"app.kubernetes.io/name":       app.Name,
				"app.kubernetes.io/part-of":    app.Org,
				"app.kubernetes.io/created-by": app.Username,
				models.EpinioStageIDLabel:      app.Stage.ID,
				models.EpinioStageBlobUIDLabel: app.BlobUID,
				"app.kubernetes.io/managed-by": "epinio",
				"app.kubernetes.io/component":  "staging",
			},
		},
		Spec: v1beta1.PipelineRunSpec{
			ServiceAccountName: "staging-triggers-admin",
			PipelineRef:        &v1beta1.PipelineRef{Name: "staging-pipeline"},
			Params: []v1beta1.Param{
				{Name: "APP_IMAGE", Value: *str(app.ImageURL(app.RegistryURL))},
				{Name: "BUILDER_IMAGE", Value: *str(app.BuilderImage)},
				{Name: "ENV_VARS", Value: v1beta1.ArrayOrString{
					Type:     v1beta1.ParamTypeArray,
					ArrayVal: app.Environment.StagingEnvArray()},
				},
				{Name: "AWS_SCRIPT", Value: *str(awsScript)},
				{Name: "AWS_ARGS", Value: v1beta1.ArrayOrString{
					Type:     v1beta1.ParamTypeArray,
					ArrayVal: awsArgs},
				},
			},
			Workspaces: []v1beta1.WorkspaceBinding{
				{
					Name:    "cache",
					SubPath: "cache",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: app.MakePVCName(),
						ReadOnly:  false,
					},
				},
				{
					Name:    "source",
					SubPath: "source",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: app.MakePVCName(),
						ReadOnly:  false,
					},
				},
				{
					Name: "s3secret",
					Secret: &corev1.SecretVolumeSource{
						SecretName: deployments.S3ConnectionDetailsSecret,
						Items: []corev1.KeyToPath{
							{Key: "config", Path: "config"},
							{Key: "credentials", Path: "credentials"},
						},
					},
				},
			},
		},
	}
}
