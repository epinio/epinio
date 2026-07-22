package application

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/randstr"
	"github.com/epinio/epinio/internal/api/v1/deploy"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/registry"
	"github.com/epinio/epinio/internal/s3manager"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

func SourcePatch(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")
	name := c.Param("app")
	username := requestctx.User(ctx).Username

	// process_cmd is the command the supervisor falls back to when no dev
	// binary is present in /epinio-sync/. By default the supervisor discovers
	// the entrypoint from /cnb/process; set this for non-CNB images (e.g.
	// "/app/bin/start", "bundle exec puma").
	processCmd := c.Request.FormValue("process_cmd")

	file, apiError := parseFile(c)
	if apiError != nil {
		return apiError
	}
	defer func() {
		fileCloseError := file.Close()
		if fileCloseError != nil {
			log.Errorw("file failed to close", "error", fileCloseError)
		}
	}()

	cluster, getClusterError := kubernetes.GetCluster(ctx)
	if getClusterError != nil {
		return apierror.InternalError(
			getClusterError,
			"failed to get access to a kube client",
		)
	}

	appRef := models.NewAppRef(name, namespace)
	newBlobUID, app, connectionDetails, apiError := sourceInfo(
		ctx,
		cluster,
		appRef,
		namespace,
		name,
		username,
		file,
		log,
	)
	if apiError != nil {
		return apiError
	}

	stageResponse, params, apiError := buildAndPatch(
		ctx,
		cluster,
		appRef,
		newBlobUID,
		app,
		connectionDetails,
		username,
	)
	if apiError != nil {
		return apiError
	}

	// We can't use the request ctx here, it is cancelled when this handler
	// returns.
	go asyncDeploy(
		cluster,
		appRef,
		&stageResponse,
		params,
		processCmd,
		name,
		namespace,
		log,
	)

	response.OKReturn(c, stageResponse)
	return nil
}

func parseFile(c *gin.Context) (multipart.File, apierror.APIErrors) {
	file, _, formFileError := c.Request.FormFile("file")

	if formFileError != nil {
		return nil, apierror.
			NewBadRequestError(formFileError.Error()).
			WithDetails("can't read multipart file input")
	}

	contentType, contentTypeError := GetFileContentType(file)
	if contentTypeError != nil {
		_ = file.Close()
		return nil, apierror.InternalError(
			contentTypeError,
			"can't detect content type of archive",
		)
	}

	if !isValidType(contentType) {
		_ = file.Close()
		return nil, apierror.NewBadRequestErrorf(
			"archive type not supported [%s]",
			contentType,
		)
	}

	return file, nil
}

func sourceInfo(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	appRef models.AppRef,
	namespace, name, username string,
	file io.Reader,
	log *zap.SugaredLogger,
) (
	string,
	*unstructured.Unstructured,
	s3manager.ConnectionDetails,
	apierror.APIErrors,
) {
	connectionDetails, getConnectionDetailsError := s3manager.GetConnectionDetails(
		ctx,
		cluster,
		helmchart.Namespace(),
		helmchart.S3ConnectionDetailsSecretName,
	)

	if getConnectionDetailsError != nil {
		return "", nil, s3manager.ConnectionDetails{}, apierror.InternalError(
			getConnectionDetailsError,
			"fetching the S3 connection details from the Kubernetes secret",
		)
	}

	manager, createManagerError := s3manager.New(connectionDetails)
	if createManagerError != nil {
		return "", nil, s3manager.ConnectionDetails{}, apierror.InternalError(
			createManagerError,
			"creating an S3 manager",
		)
	}

	app, getAppError := application.Get(ctx, cluster, appRef)
	if getAppError != nil {
		return "", nil, s3manager.ConnectionDetails{}, apierror.InternalError(
			getAppError,
			"failed to get the application resource",
		)
	}

	currentBlobUID, findBlobError := findPreviousBlobUID(app)
	if findBlobError != nil {
		return "", nil, s3manager.ConnectionDetails{}, apierror.InternalError(
			findBlobError,
			"failed to find existing blob UID",
		)
	}
	if currentBlobUID == "" {
		return "", nil, s3manager.ConnectionDetails{}, apierror.NewBadRequestError(
			"no existing source blob found,  push the app at least once before using watch",
		)
	}

	existingBlob, downloadBlobError := manager.Download(ctx, currentBlobUID)
	if downloadBlobError != nil {
		return "", nil, s3manager.ConnectionDetails{}, apierror.InternalError(
			downloadBlobError,
			"downloading existing source blob",
		)
	}
	defer func() {
		closeError := existingBlob.Close()
		if closeError != nil {
			log.Errorw("failed to close existing blob reader", "error", closeError)
		}
	}()

	patchedBlob, applyPatchError := applySourcePatch(existingBlob, file)
	if applyPatchError != nil {
		return "", nil, s3manager.ConnectionDetails{}, apierror.InternalError(
			applyPatchError,
			"applying the source patch",
		)
	}

	newBlobUID, uploadBlobError := manager.UploadStream(
		ctx,
		patchedBlob,
		-1,
		map[string]string{
			"app": name, "namespace": namespace, "username": username,
		},
	)
	if uploadBlobError != nil {
		return "", nil, s3manager.ConnectionDetails{}, apierror.InternalError(
			uploadBlobError,
			"uploading the patched application sources blob",
		)
	}

	log.Infow(
		"uploaded patched source",
		"namespace",
		namespace,
		"app",
		name,
		"blobUID",
		newBlobUID,
	)

	return newBlobUID, app, connectionDetails, nil
}

func buildAndPatch(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	appRef models.AppRef,
	newBlobUID string,
	app *unstructured.Unstructured,
	connectionDetails s3manager.ConnectionDetails,
	username string,
) (models.StageResponse, stageParam, apierror.APIErrors) {
	req := models.StageRequest{
		App:     appRef,
		BlobUID: newBlobUID,
	}

	builderImage, builderError := getBuilderImage(req, app)
	if builderError != nil {
		return models.StageResponse{}, stageParam{}, builderError
	}
	if builderImage == "" {
		builderImage = viper.GetString("default-builder-image")
	}

	config, determineStagingError := DetermineStagingScripts(
		ctx,
		cluster,
		helmchart.Namespace(),
		builderImage,
	)
	if determineStagingError != nil {
		return models.StageResponse{}, stageParam{}, apierror.InternalError(
			determineStagingError,
			"failed to retrieve staging configuration",
		)
	}

	environment, getEnvironmentError := application.Environment(ctx, cluster, appRef)
	if getEnvironmentError != nil {
		return models.StageResponse{}, stageParam{}, apierror.InternalError(
			getEnvironmentError,
			"failed to access application runtime environment",
		)
	}

	for envName, value := range config.Env {
		if _, found := environment[envName]; found {
			continue
		}
		environment[envName] = value
	}

	owner := metav1.OwnerReference{
		APIVersion: app.GetAPIVersion(),
		Kind:       app.GetKind(),
		Name:       app.GetName(),
		UID:        app.GetUID(),
	}

	previousID, getStageIDError := application.StageID(app)
	if getStageIDError != nil {
		return models.StageResponse{}, stageParam{}, apierror.InternalError(
			getStageIDError,
			"failed to determine application stage id",
		)
	}

	uid, generateUIDError := randstr.Hex16()
	if generateUIDError != nil {
		return models.StageResponse{}, stageParam{}, apierror.InternalError(
			generateUIDError,
			"failed to generate a uid",
		)
	}
	if previousID == "" {
		previousID = uid
	}

	registryPublicURL, getRegistryURLError := getRegistryURL(ctx, cluster)
	if getRegistryURLError != nil {
		return models.StageResponse{}, stageParam{}, apierror.InternalError(
			getRegistryURLError,
			"getting the Epinio registry public URL",
		)
	}

	registryCertificateSecret,
		registryCACertKey,
		registryCertificateHash,
		getCertHashError := resolveRegistryCertHash(ctx, cluster)
	if getCertHashError != nil {
		return models.StageResponse{}, stageParam{}, apierror.InternalError(
			getCertHashError,
			"cannot calculate Certificate hash",
		)
	}

	params := stageParam{
		AppRef:              appRef,
		BuilderImage:        builderImage,
		BlobUID:             newBlobUID,
		DownloadImage:       config.DownloadImage,
		UnpackImage:         config.UnpackImage,
		Environment:         environment.List(),
		Owner:               owner,
		RegistryURL:         registryPublicURL,
		S3ConnectionDetails: connectionDetails,
		Stage:               models.NewStage(uid),
		PreviousStageID:     previousID,
		Username:            username,
		RegistryCAHash:      registryCertificateHash,
		RegistryCASecret:    registryCertificateSecret,
		RegistryCACertKey:   registryCACertKey,
		UserID:              config.UserID,
		GroupID:             config.GroupID,
		Scripts:             config.Name,
		HelmValues:          config.HelmValues,
	}

	stageResponse, stageErr := executeStage(ctx, cluster, app, req, params)
	if stageErr != nil {
		return models.StageResponse{}, stageParam{}, stageErr
	}

	return stageResponse, params, nil
}

func asyncDeploy(
	cluster *kubernetes.Cluster,
	appRef models.AppRef,
	stageResponse *models.StageResponse,
	params stageParam,
	processCmd, name, namespace string,
	log *zap.SugaredLogger,
) {
	bgCtx := context.Background()
	bgLog := log.With(
		"stageID",
		params.Stage.ID,
		"app",
		name,
		"namespace",
		namespace,
	)
	totalStart := time.Now()

	t := time.Now()
	jobs, jobsError := stageJobs(bgCtx, cluster, namespace, params.Stage.ID)
	if jobsError != nil {
		bgLog.Errorw("failed to list staging jobs", "error", jobsError)
		return
	}
	bgLog.Infow("step:stageJobs", "ms", time.Since(t).Milliseconds())

	t = time.Now()
	success, waitStagingError := waitForStagingCompletion(bgCtx, cluster, jobs)
	if waitStagingError != nil {
		bgLog.Errorw(
			"error waiting for staging completion",
			"error",
			waitStagingError,
		)
		return
	}
	if !success {
		bgLog.Errorw("staging failed", "stageID", params.Stage.ID)
		return
	}
	bgLog.Infow("step:staging", "ms", time.Since(t).Milliseconds())

	t = time.Now()
	freshApp, getAppError := application.Get(bgCtx, cluster, appRef)
	if getAppError != nil {
		bgLog.Errorw("failed to re-fetch app CR for deploy", "error", getAppError)
		return
	}
	bgLog.Infow("step:fetchApp", "ms", time.Since(t).Milliseconds())

	t = time.Now()
	updateImageError := deploy.UpdateImageURL(
		bgCtx,
		cluster,
		freshApp,
		stageResponse.ImageURL,
	)
	if updateImageError != nil {
		bgLog.Errorw("failed to update image URL", "error", updateImageError)
		return
	}
	bgLog.Infow("step:updateImageURL", "ms", time.Since(t).Milliseconds())

	// Transform public registry URL (cluster-internal DNS) to the NodePort URL
	// that the kubelet can resolve when pulling the image.
	registryDetails, rdError := registry.GetConnectionDetails(
		bgCtx,
		cluster,
		helmchart.Namespace(),
		registry.CredentialsSecretName,
	)
	if rdError != nil {
		bgLog.Errorw("failed to get registry connection details", "error", rdError)
		return
	}

	deployImageURL, rdError := registryDetails.ReplaceWithInternalRegistry(
		stageResponse.ImageURL,
	)
	if rdError != nil {
		bgLog.Errorw("failed to replace registry URL", "error", rdError)
		return
	}

	t = time.Now()
	swapImageError := swapPodImage(
		bgCtx,
		cluster,
		appRef,
		deployImageURL,
		params.UserID,
		params.GroupID,
		processCmd,
	)
	if swapImageError != nil {
		bgLog.Errorw("pod image swap failed", "error", swapImageError)
		return
	}
	bgLog.Infow("step:swapPodImage", "ms", time.Since(t).Milliseconds())

	bgLog.Infow(
		"watch-deploy complete",
		"image",
		stageResponse.ImageURL,
		"total_ms",
		time.Since(totalStart).Milliseconds(),
	)
}

// swapPodImage bypasses Helm: patches the deployment image and securityContext
// directly then deletes the running pod so k8s immediately recreates it.
// The securityContext is set to the build UID/GID so that subsequent file
// syncs (kubectl exec tar) can write to /workspace/source/app, which is owned
// by the build user.
func swapPodImage(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	appRef models.AppRef,
	imageURL string,
	userID,
	groupID int64,
	processCmd string,
) error {
	labelSelector := fmt.Sprintf("app.kubernetes.io/name=%s", appRef.Name)

	deployments, getDeploymentsError := cluster.
		Kubectl.
		AppsV1().
		Deployments(appRef.Namespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
	if getDeploymentsError != nil {
		return fmt.Errorf(
			"listing deployments for app %s: %w",
			appRef.Name,
			getDeploymentsError,
		)
	}
	if len(deployments.Items) == 0 {
		return fmt.Errorf("deployment not found for app %s", appRef.Name)
	}
	d := deployments.Items[0]
	if len(d.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("deployment %s has no containers", d.Name)
	}
	containerName := d.Spec.Template.Spec.Containers[0].Name

	// Supervisor loop runs as PID 1 so k8s never counts a container restart.
	// Each iteration checks for a dev binary in /epinio-sync/app first, then
	// falls back to processCmd (the app's normal entrypoint). The child PID is
	// written to /epinio-sync/pid so the sync handler can reload without
	// restarting the container.
	// By default the entrypoint is discovered from /cnb/process: "web" when
	// the buildpack defines it (PHP, Node, Procfile apps), otherwise the
	// first process symlink (the Paketo Go buildpack names the process after
	// the module, so /cnb/process/web does not exist there). Set processCmd
	// for non-CNB images or to pick a specific process.
	//
	// The launch is wrapped in setsid so the app runs in its own process
	// group whose id equals the recorded PID. Entrypoints that fork or
	// background the real server (e.g. a start.sh doing `( node app ) &`)
	// would otherwise leave the recorded PID pointing at a wrapper shell;
	// the sync handler kills the whole group (kill -9 -PID) so the real
	// server dies and frees its port for the relaunch. Without this, an
	// orphaned server keeps the port and the reloaded process can't bind.
	if processCmd == "" {
		processCmd = `"$APP_CMD"`
	}
	wrapperCmd := fmt.Sprintf(
		`APP_CMD=/cnb/process/web; [ -x "$APP_CMD" ] || APP_CMD="$(ls /cnb/process/* 2>/dev/null | head -n1)"; while true; do if [ -f /epinio-sync/app ]; then setsid /epinio-sync/app & else setsid %s & fi; echo $! > /epinio-sync/pid; wait; sleep 0.1; done`,
		processCmd,
	)
	patch := []byte(fmt.Sprintf(
		`{"spec":{"template":{"spec":{`+
			`"securityContext":{"runAsUser":%d,"runAsGroup":%d},`+
			`"volumes":[{"name":"epinio-sync","emptyDir":{}}],`+
			`"containers":[{"name":%q,"image":%q,`+
			`"command":["sh","-c",%q],`+
			`"volumeMounts":[{"name":"epinio-sync","mountPath":"/epinio-sync"}]}]`+
			`}}}}`,
		userID, groupID, containerName, imageURL, wrapperCmd,
	))

	_, patchError := cluster.
		Kubectl.
		AppsV1().
		Deployments(appRef.Namespace).
		Patch(
			ctx,
			d.Name,
			types.StrategicMergePatchType,
			patch,
			metav1.PatchOptions{},
		)
	if patchError != nil {
		return fmt.Errorf("patching deployment image: %w", patchError)
	}

	// Delete the running pod -- k8s recreates it immediately with the new image
	pods, getPodsError := cluster.Kubectl.CoreV1().Pods(appRef.Namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: "status.phase=Running",
		},
	)
	if getPodsError != nil {
		return fmt.Errorf(
			"listing pods for app %s: %w",
			appRef.Name,
			getPodsError,
		)
	}
	if len(pods.Items) == 0 {
		return nil // no running pod; deployment patch is enough
	}

	return cluster.Kubectl.CoreV1().Pods(appRef.Namespace).Delete(
		ctx,
		pods.Items[0].Name,
		metav1.DeleteOptions{},
	)
}

func extractTar(archive io.Reader) (map[string][]byte, error) {
	data, readAllError := io.ReadAll(archive)
	if readAllError != nil {
		return nil, readAllError
	}

	// Detect gzip vs plain tar, epinio push stores plain tar, but client patches
	// may be gzip.extractTar
	var tarReader *tar.Reader
	gz, gzError := gzip.NewReader(bytes.NewReader(data))
	if gzError == nil {
		defer func() {
			closeError := gz.Close()
			if closeError != nil {
				log.Printf("error closing gzip reader: %v", closeError)
			}
		}()
		tarReader = tar.NewReader(gz)
	} else {
		tarReader = tar.NewReader(bytes.NewReader(data))
	}

	files := make(map[string][]byte)

	for {
		header, tarError := tarReader.Next()
		if tarError == io.EOF {
			break
		}
		if tarError != nil {
			return nil, tarError
		}

		if header.Typeflag == tar.TypeReg {
			content, readError := io.ReadAll(tarReader)
			if readError != nil {
				return nil, readError
			}
			// normalize leading ./ so patch keys match base blob keys
			files[strings.TrimPrefix(header.Name, "./")] = content
		}
	}

	return files, nil
}

func createTar(files map[string][]byte) (io.Reader, error) {
	var buffer bytes.Buffer
	tarWriter := tar.NewWriter(&buffer)

	for name, content := range files {
		header := &tar.Header{
			Name:     name,
			Mode:     0644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}

		headerWriteError := tarWriter.WriteHeader(header)
		if headerWriteError != nil {
			return nil, headerWriteError
		}

		_, writeError := tarWriter.Write(content)
		if writeError != nil {
			return nil, writeError
		}
	}

	closeError := tarWriter.Close()
	if closeError != nil {
		return nil, closeError
	}

	return bytes.NewReader(buffer.Bytes()), nil
}

func applySourcePatch(base io.Reader, patch io.Reader) (io.Reader, error) {
	baseFiles, baseFilesError := extractTar(base)
	if baseFilesError != nil {
		return nil, baseFilesError
	}

	patchFiles, patchFilesError := extractTar(patch)
	if patchFilesError != nil {
		return nil, patchFilesError
	}

	for name, content := range patchFiles {
		baseFiles[name] = content
	}

	return createTar(baseFiles)
}
