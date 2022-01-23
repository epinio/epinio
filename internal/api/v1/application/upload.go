package application

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/s3manager"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// Upload handles the API endpoint /namespaces/:namespace/applications/:app/store.
// It receives the application data as a tarball and stores it. Then
// it creates the k8s resources needed for staging
func (hc Controller) Upload(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := tracelog.Logger(ctx)

	namespace := c.Param("namespace")
	name := c.Param("app")

	log.Info("processing upload", "namespace", namespace, "app", name)

	log.V(2).Info("parsing multipart form")

	file, fileheader, err := c.Request.FormFile("file")
	if err != nil {
		return apierror.BadRequest(err, "can't read multipart file input")
	}
	defer file.Close()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err, "failed to get access to a kube client")
	}

	connectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster, helmchart.StagingNamespace, helmchart.S3ConnectionDetailsSecretName)
	if err != nil {
		return apierror.InternalError(err, "fetching the S3 connection details from the Kubernetes secret")
	}
	manager, err := s3manager.New(connectionDetails)
	if err != nil {
		return apierror.InternalError(err, "creating an S3 manager")
	}

	username := requestctx.User(ctx)
	blobUID, err := manager.UploadStream(ctx, file, fileheader.Size, map[string]string{
		"app": name, "namespace": namespace, "username": username,
	})
	if err != nil {
		return apierror.InternalError(err, "uploading the application sources blob")
	}

	log.Info("uploaded app", "namespace", namespace, "app", name, "blobUID", blobUID)

	response.OKReturn(c, models.UploadResponse{
		BlobUID: blobUID,
	})
	return nil
}
