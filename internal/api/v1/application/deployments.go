// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/s3manager"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

type asyncDeployJob struct {
	status models.AsyncDeployStatus
}

var (
	asyncDeployJobsMu sync.RWMutex
	asyncDeployJobs   = map[string]*asyncDeployJob{}
)

func asyncDeployJobID() (string, error) {
	return randstr.Hex16()
}

// DeploymentsStart handles POST /namespaces/:namespace/applications/:app/deployments
//
// It starts a background flow that stages/builds (when BlobUID is provided) and deploys the application,
// and returns immediately with a deployment id that can be used to query status.
func DeploymentsStart(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()

	namespace := c.Param("namespace")
	name := c.Param("app")

	req := models.AsyncDeployRequest{}
	if err := c.BindJSON(&req); err != nil {
		return apierror.NewBadRequestError(err.Error()).WithDetails("failed to unmarshal async deploy request")
	}
	if name != req.App.Name {
		return apierror.NewBadRequestError("name parameter from URL does not match name param in body")
	}
	if namespace != req.App.Namespace {
		return apierror.NewBadRequestError("namespace parameter from URL does not match namespace param in body")
	}
	if req.ImageURL == "" && req.BlobUID == "" {
		return apierror.NewBadRequestError("async deploy requires either `image` or `blobuid`")
	}

	id, err := asyncDeployJobID()
	if err != nil {
		return apierror.InternalError(err, "failed to generate async deploy id")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	job := &asyncDeployJob{
		status: models.AsyncDeployStatus{
			ID:        id,
			App:       req.App,
			Status:    "pending",
			StartedAt: now,
		},
	}

	asyncDeployJobsMu.Lock()
	asyncDeployJobs[id] = job
	asyncDeployJobsMu.Unlock()

	username := requestctx.User(ctx).Username
	go runAsyncDeployment(context.Background(), id, req, username)

	c.JSON(202, job.status)
	return nil
}

// DeploymentsStatus handles GET /namespaces/:namespace/applications/:app/deployments/:deployment_id
func DeploymentsStatus(c *gin.Context) apierror.APIErrors {
	deploymentID := c.Param("deployment_id")

	asyncDeployJobsMu.RLock()
	job, ok := asyncDeployJobs[deploymentID]
	asyncDeployJobsMu.RUnlock()

	if !ok {
		return apierror.NewNotFoundError("deployment", deploymentID)
	}

	response.OKReturn(c, job.status)
	return nil
}

func runAsyncDeployment(ctx context.Context, deploymentID string, req models.AsyncDeployRequest, username string) {
	log := requestctx.Logger(ctx).With("component", "async-deploy", "deploymentID", deploymentID)

	update := func(mut func(*models.AsyncDeployStatus)) {
		asyncDeployJobsMu.Lock()
		defer asyncDeployJobsMu.Unlock()

		job, ok := asyncDeployJobs[deploymentID]
		if !ok {
			return
		}
		mut(&job.status)
	}

	failErr := func(err error) {
		update(func(s *models.AsyncDeployStatus) {
			s.Status = "failed"
			s.Error = err.Error()
			s.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		})
	}

	failAPI := func(apiErr apierror.APIErrors) {
		msg := "request failed"
		if apiErr != nil {
			errs := apiErr.Errors()
			if len(errs) > 0 && errs[0].Title != "" {
				msg = errs[0].Title
			}
		}
		update(func(s *models.AsyncDeployStatus) {
			s.Status = "failed"
			s.Error = msg
			s.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		})
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		log.Errorw("failed to get cluster", "error", err)
		failErr(err)
		return
	}

	var stageID string
	var imageURL string

	// Stage/build when we have a blob uid. Otherwise deploy the provided image.
	if req.BlobUID != "" {
		update(func(s *models.AsyncDeployStatus) { s.Status = "staging" })

		stageResp, apiErr := stageForAsyncDeploy(ctx, cluster, req.App, req.BlobUID, req.BuilderImage, username)
		if apiErr != nil {
			failAPI(apiErr)
			return
		}

		stageID = stageResp.Stage.ID
		imageURL = stageResp.ImageURL
		update(func(s *models.AsyncDeployStatus) {
			s.StageID = stageID
			s.ImageURL = imageURL
		})

		jobs, apiErr := stageJobs(ctx, cluster, req.App.Namespace, stageID)
		if apiErr != nil {
			failAPI(apiErr)
			return
		}
		success, waitErr := waitForStagingCompletion(ctx, cluster, jobs)
		if waitErr != nil {
			log.Errorw("staging completion wait failed", "error", waitErr)
			failErr(waitErr)
			return
		}
		if !success {
			failAPI(apierror.NewInternalError("Failed to stage", "staging job failed"))
			return
		}
	} else {
		imageURL = req.ImageURL
		update(func(s *models.AsyncDeployStatus) { s.ImageURL = imageURL })
	}

	update(func(s *models.AsyncDeployStatus) { s.Status = "deploying" })

	applicationCR, err := application.Get(ctx, cluster, req.App)
	if err != nil {
		if apierrors.IsNotFound(err) {
			failAPI(apierror.AppIsNotKnown("cannot deploy app, application resource is missing"))
			return
		}
		failErr(err)
		return
	}

	if err := deploy.UpdateImageURL(ctx, cluster, applicationCR, imageURL); err != nil {
		failErr(err)
		return
	}

	desiredRoutes, found, err := unstructured.NestedStringSlice(applicationCR.Object, "spec", "routes")
	if err != nil {
		failErr(err)
		return
	}
	if !found {
		desiredRoutes = []string{}
	}

	if apiErr := validateRoutes(ctx, cluster, req.App.Name, req.App.Namespace, desiredRoutes); apiErr != nil {
		failAPI(apiErr)
		return
	}

	deployResult, apiErr := deploy.DeployApp(ctx, cluster, req.App, username, stageID)
	if apiErr != nil {
		failAPI(apiErr)
		return
	}

	if err := application.SetOrigin(ctx, cluster, req.App, req.Origin); err != nil {
		failErr(err)
		return
	}

	update(func(s *models.AsyncDeployStatus) {
		s.Status = "succeeded"
		s.Routes = deployResult.Routes
		s.Warnings = deployResult.Warnings
		s.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	})
}

// stageForAsyncDeploy performs the same staging operation as Stage() but without a gin context.
func stageForAsyncDeploy(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	appRef models.AppRef,
	blobUID string,
	builderImage string,
	username string,
) (*models.StageResponse, apierror.APIErrors) {
	log := requestctx.Logger(ctx).With("component", "async-stage")

	// check application resource
	app, err := application.Get(ctx, cluster, appRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierror.AppIsNotKnown("cannot stage app, application resource is missing")
		}
		return nil, apierror.InternalError(err, "failed to get the application resource")
	}

	// quickly reject conflict with (still) active staging
	staging, err := application.IsCurrentlyStaging(ctx, cluster, appRef.Namespace, appRef.Name)
	if err != nil {
		return nil, apierror.InternalError(err)
	}
	if staging {
		return nil, apierror.NewBadRequestError("staging job for image ID still running")
	}

	// determine builder image (request overrides)
	stageReq := models.StageRequest{
		App:         appRef,
		BlobUID:     blobUID,
		BuilderImage: builderImage,
	}

	builder, builderErr := getBuilderImage(stageReq, app)
	if builderErr != nil {
		return nil, builderErr
	}
	if builder == "" {
		builder = viper.GetString("default-builder-image")
		if builder == "" {
			return nil, apierror.NewBadRequestError("no builder image specified and no default configured")
		}
	}

	config, err := DetermineStagingScripts(ctx, cluster, helmchart.Namespace(), builder)
	if err != nil {
		return nil, apierror.InternalError(err, "failed to retrieve staging configuration")
	}

	log.Infow("staging app", "scripts", config.Name, "builder", builder)

	s3ConnectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster,
		helmchart.Namespace(), helmchart.S3ConnectionDetailsSecretName)
	if err != nil {
		return nil, apierror.InternalError(err, "failed to fetch the S3 connection details")
	}

	// Validate incoming blob id before attempting to stage (reuse existing helper)
	if apiErr := validateBlob(ctx, blobUID, appRef, s3ConnectionDetails); apiErr != nil {
		return nil, apiErr
	}

	uid, err := randstr.Hex16()
	if err != nil {
		return nil, apierror.InternalError(err, "failed to generate a uid")
	}

	environment, err := application.Environment(ctx, cluster, appRef)
	if err != nil {
		return nil, apierror.InternalError(err, "failed to access application runtime environment")
	}

	owner := metav1.OwnerReference{
		APIVersion: app.GetAPIVersion(),
		Kind:       app.GetKind(),
		Name:       app.GetName(),
		UID:        app.GetUID(),
	}

	previousID, err := application.StageID(app)
	if err != nil {
		return nil, apierror.InternalError(err, "failed to determine application stage id")
	}
	if previousID == "" {
		previousID = uid
	}

	registryPublicURL, err := getRegistryURL(ctx, cluster)
	if err != nil {
		return nil, apierror.InternalError(err, "getting the Epinio registry public URL")
	}

	registryCertificateSecret := viper.GetString("registry-certificate-secret")
	registryCertificateHash := ""
	if registryCertificateSecret != "" {
		registryCertificateHash, err = getRegistryCertificateHash(ctx, cluster, helmchart.Namespace(), registryCertificateSecret)
		if err != nil {
			return nil, apierror.InternalError(err, "cannot calculate Certificate hash")
		}
	}

	for name, value := range config.Env {
		if _, found := environment[name]; found {
			continue
		}
		environment[name] = value
	}

	params := stageParam{
		AppRef:              appRef,
		BuilderImage:        builder,
		DownloadImage:       config.DownloadImage,
		UnpackImage:         config.UnpackImage,
		BlobUID:             blobUID,
		Environment:         environment.List(),
		Owner:               owner,
		RegistryURL:         registryPublicURL,
		S3ConnectionDetails: s3ConnectionDetails,
		Stage:               models.NewStage(uid),
		PreviousStageID:     previousID,
		Username:            username,
		RegistryCAHash:      registryCertificateHash,
		RegistryCASecret:    registryCertificateSecret,
		UserID:              config.UserID,
		GroupID:             config.GroupID,
		Scripts:             config.Name,
		HelmValues:          config.HelmValues,
	}

	if !params.HelmValues.Storage.Cache.EmptyDir {
		if err := ensurePVC(ctx, cluster, params.HelmValues.Storage.Cache, appRef.MakeCachePVCName()); err != nil {
			return nil, apierror.InternalError(err, "failed to ensure a PersistentVolumeClaim for the application cache")
		}
	}
	if !params.HelmValues.Storage.SourceBlobs.EmptyDir {
		if err := ensurePVC(ctx, cluster, params.HelmValues.Storage.SourceBlobs, appRef.MakeSourceBlobsPVCName()); err != nil {
			return nil, apierror.InternalError(err, "failed to ensure a PersistentVolumeClaim for the application source blobs")
		}
	}

	job, jobenv := newJobRun(params)

	if err := cluster.CreateSecret(ctx, helmchart.Namespace(), *jobenv); err != nil {
		return nil, apierror.InternalError(errors.Wrap(err, "failed to create job env secret"))
	}
	if err := cluster.CreateJob(ctx, helmchart.Namespace(), job); err != nil {
		return nil, apierror.InternalError(errors.Wrap(err, "failed to create staging job"))
	}
	if err := updateApp(ctx, cluster, app, params); err != nil {
		return nil, apierror.InternalError(err, "updating application CR with staging information")
	}

	imageURL := params.ImageURL(params.RegistryURL)
	return &models.StageResponse{
		Stage:    models.NewStage(uid),
		ImageURL: imageURL,
	}, nil
}

