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
	"io"
	"mime/multipart"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/s3manager"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/h2non/filetype"
	"github.com/pkg/errors"
)

// Should match the supported types:
// https://github.com/epinio/helm-charts/blob/3954c214de3d7b957cfc2054ba4fa4bfa140f5a3/chart/epinio/templates/stage-scripts.yaml#L27-L62
// This is what the filetype library supports: https://github.com/h2non/filetype#supported-types
var validArchiveTypes = []string{
	"application/zip",
	"application/x-tar",
	"application/gzip",
	"application/x-bzip2",
	"application/x-xz",
}

// Upload handles the API endpoint /namespaces/:namespace/applications/:app/store.
// It receives the application data as an archive (tarball, zip, ...) and stores it.
// Then it creates the k8s resources needed for staging
func Upload(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")
	name := c.Param("app")

	log.Info("processing upload", "namespace", namespace, "app", name)

	log.V(2).Info("parsing multipart form")

	file, fileheader, err := c.Request.FormFile("file")
	if err != nil {
		return apierror.NewBadRequestError(err.Error()).WithDetails("can't read multipart file input")
	}
	defer file.Close()

	// TODO: Does this break streaming of the file? We need to get the whole file
	// before we can check its type
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

	username := requestctx.User(ctx).Username
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

func GetFileContentType(file multipart.File) (string, error) {
	// to sniff the content type only the first 512 bytes are used.
	buf := make([]byte, 512)

	_, err := file.Read(buf)
	if err != nil {
		return "", errors.Wrap(err, "reading file content type")
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return "", errors.Wrap(err, "resetting file cursor after reading content type")
	}

	kind, _ := filetype.Match(buf)
	if kind == filetype.Unknown {
		return "", errors.Wrap(err, "reading the file type")
	}

	return kind.MIME.Value, nil
}

func isValidType(contentType string) bool {
	for _, validType := range validArchiveTypes {
		if contentType == validType {
			return true
		}
	}
	return false
}
