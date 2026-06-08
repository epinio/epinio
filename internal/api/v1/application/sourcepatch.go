package application

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	log.Debugw("parsing multipart form")

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		return apierror.
			NewBadRequestError(err.Error()).
			WithDetails("can't read multipart file input")
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Errorw("file failed to close", "error", err)
		}
	}()

	contentType, err := GetFileContentType(file)
	if err != nil {
		return apierror.InternalError(err, "can't detect content type of archive")
	}
	if !isValidType(contentType) {
		return apierror.NewBadRequestErrorf("archive type not supported [%s]", contentType)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}

	connectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster, helmchart.Namespace(), helmchart.S3ConnectionDetailsSecretName)
	if err != nil {
		return apierror.InternalError(err, "fetching the S3 connection details from the Kubernetes secret")
	}
	manager, err := s3manager.New(connectionDetails)
	if err != nil {
		return apierror.InternalError(err, "creating an S3 manager")
	}

	appRef := models.NewAppRef(name, namespace)
	app, err := application.Get(ctx, cluster, appRef)
	if err != nil {
		return apierror.InternalError(err, "failed to get the application resource")
	}

	currentBlobUID, err := findPreviousBlobUID(app)
	if err != nil {
		return apierror.InternalError(err, "failed to find existing blob UID")
	}
	if currentBlobUID == "" {
		return apierror.NewBadRequestError("no existing source blob found -- push the app at least once before using watch")
	}

	existingBlob, err := manager.Download(ctx, currentBlobUID)
	if err != nil {
		return apierror.InternalError(err, "downloading existing source blob")
	}
	defer existingBlob.Close()

	patchedBlob, err := applySourcePatch(existingBlob, file)
	if err != nil {
		return apierror.InternalError(err, "applying the source patch")
	}

	newBlobUID, err := manager.UploadStream(ctx, patchedBlob, -1, map[string]string{
		"app": name, "namespace": namespace, "username": username,
	})
	if err != nil {
		return apierror.InternalError(err, "uploading the patched application sources blob")
	}

	log.Infow("uploaded patched source", "namespace", namespace, "app", name, "blobUID", newBlobUID)

	req := models.StageRequest{
		App:     appRef,
		BlobUID: newBlobUID,
	}

	builderImage, builderErr := getBuilderImage(req, app)
	if builderErr != nil {
		return builderErr
	}
	if builderImage == "" {
		builderImage = viper.GetString("default-builder-image")
	}

	config, err := DetermineStagingScripts(ctx, cluster, helmchart.Namespace(), builderImage)
	if err != nil {
		return apierror.InternalError(err, "failed to retrieve staging configuration")
	}

	environment, err := application.Environment(ctx, cluster, appRef)
	if err != nil {
		return apierror.InternalError(err, "failed to access application runtime environment")
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

	previousID, err := application.StageID(app)
	if err != nil {
		return apierror.InternalError(err, "failed to determine application stage id")
	}

	uid, err := randstr.Hex16()
	if err != nil {
		return apierror.InternalError(err, "failed to generate a uid")
	}
	if previousID == "" {
		previousID = uid
	}

	registryPublicURL, err := getRegistryURL(ctx, cluster)
	if err != nil {
		return apierror.InternalError(err, "getting the Epinio registry public URL")
	}

	registryCertificateSecret, registryCACertKey, registryCertificateHash, err := resolveRegistryCertHash(ctx, cluster)
	if err != nil {
		return apierror.InternalError(err, "cannot calculate Certificate hash")
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
		return stageErr
	}

	// Deploy asynchronously after staging completes.
	// We can't use the request ctx here -- it is cancelled when this handler returns.
	go func() {
		bgCtx := context.Background()
		bgLog := log.With("stageID", params.Stage.ID, "app", name, "namespace", namespace)
		totalStart := time.Now()

		t := time.Now()
		jobs, jobsErr := stageJobs(bgCtx, cluster, namespace, params.Stage.ID)
		if jobsErr != nil {
			bgLog.Errorw("failed to list staging jobs", "error", jobsErr)
			return
		}
		bgLog.Infow("step:stageJobs", "ms", time.Since(t).Milliseconds())

		t = time.Now()
		success, err := waitForStagingCompletion(bgCtx, cluster, jobs)
		if err != nil {
			bgLog.Errorw("error waiting for staging completion", "error", err)
			return
		}
		if !success {
			bgLog.Errorw("staging failed", "stageID", params.Stage.ID)
			return
		}
		bgLog.Infow("step:staging", "ms", time.Since(t).Milliseconds())

		t = time.Now()
		freshApp, err := application.Get(bgCtx, cluster, appRef)
		if err != nil {
			bgLog.Errorw("failed to re-fetch app CR for deploy", "error", err)
			return
		}
		bgLog.Infow("step:fetchApp", "ms", time.Since(t).Milliseconds())

		t = time.Now()
		if err := deploy.UpdateImageURL(bgCtx, cluster, freshApp, stageResponse.ImageURL); err != nil {
			bgLog.Errorw("failed to update image URL", "error", err)
			return
		}
		bgLog.Infow("step:updateImageURL", "ms", time.Since(t).Milliseconds())

		// Transform public registry URL (cluster-internal DNS) to the NodePort URL
		// that the kubelet can resolve when pulling the image.
		registryDetails, rdErr := registry.GetConnectionDetails(bgCtx, cluster, helmchart.Namespace(), registry.CredentialsSecretName)
		if rdErr != nil {
			bgLog.Errorw("failed to get registry connection details", "error", rdErr)
			return
		}
		deployImageURL, rdErr := registryDetails.ReplaceWithInternalRegistry(stageResponse.ImageURL)
		if rdErr != nil {
			bgLog.Errorw("failed to replace registry URL", "error", rdErr)
			return
		}

		t = time.Now()
		if err := swapPodImage(bgCtx, cluster, appRef, deployImageURL, params.UserID, params.GroupID); err != nil {
			bgLog.Errorw("pod image swap failed", "error", err)
			return
		}
		bgLog.Infow("step:swapPodImage", "ms", time.Since(t).Milliseconds())

		bgLog.Infow("watch-deploy complete", "image", stageResponse.ImageURL, "total_ms", time.Since(totalStart).Milliseconds())
	}()

	response.OKReturn(c, stageResponse)
	return nil
}

// swapPodImage bypasses Helm: patches the deployment image and securityContext directly
// then deletes the running pod so k8s immediately recreates it. The securityContext is set
// to the build UID/GID so that subsequent file syncs (kubectl exec tar) can write to
// /workspace/source/app, which is owned by the build user.
func swapPodImage(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef, imageURL string, userID, groupID int64) error {
	labelSelector := fmt.Sprintf("app.kubernetes.io/name=%s", appRef.Name)

	// Patch the deployment image
	deployments, err := cluster.Kubectl.AppsV1().Deployments(appRef.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil || len(deployments.Items) == 0 {
		return fmt.Errorf("deployment not found for app %s", appRef.Name)
	}
	d := deployments.Items[0]
	if len(d.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("deployment %s has no containers", d.Name)
	}
	containerName := d.Spec.Template.Spec.Containers[0].Name

	// The wrapper command checks for a dev binary in the epinio-sync emptyDir volume first,
	// falling back to the normal Paketo launcher. This enables binary swap for compiled
	// languages without requiring changes after the dev session ends -- any pod restart
	// with an empty emptyDir will transparently use the original image binary.
	// Supervisor loop always runs as PID 1 so k8s never sees a container restart.
	// Each iteration picks /epinio-sync/app if present, else falls back to /cnb/process/web.
	// The child PID is written to /epinio-sync/pid on every iteration so the sync script
	// can kill just the child to trigger a hot reload without restarting the container.
	const wrapperCmd = `while true; do if [ -f /epinio-sync/app ]; then /epinio-sync/app & else /cnb/process/web & fi; echo $! > /epinio-sync/pid; wait; sleep 0.1; done`
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
	if _, err := cluster.Kubectl.AppsV1().Deployments(appRef.Namespace).Patch(
		ctx, d.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{},
	); err != nil {
		return fmt.Errorf("patching deployment image: %w", err)
	}

	// Delete the running pod -- k8s recreates it immediately with the new image
	pods, err := cluster.Kubectl.CoreV1().Pods(appRef.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: "status.phase=Running",
	})
	if err != nil || len(pods.Items) == 0 {
		return nil // no running pod, deployment patch is enough
	}
	return cluster.Kubectl.CoreV1().Pods(appRef.Namespace).Delete(
		ctx, pods.Items[0].Name, metav1.DeleteOptions{},
	)
}

func extractTar(archive io.Reader) (map[string][]byte, error) {
	data, err := io.ReadAll(archive)
	if err != nil {
		return nil, err
	}

	// Detect gzip vs plain tar -- epinio push stores plain tar, but client patches may be gzip
	var tarReader *tar.Reader
	if gz, gzErr := gzip.NewReader(bytes.NewReader(data)); gzErr == nil {
		defer gz.Close()
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

		_ = tarWriter.WriteHeader(header)
		_, _ = tarWriter.Write(content)
	}

	_ = tarWriter.Close()

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
