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
	"bufio"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/s3manager"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// GetSource handles GET /namespaces/:namespace/applications/:app/source.
// It returns the application's staging source tarball from S3.
func GetSource(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")
	appName := c.Param("app")

	log.Infow("fetching app source", "namespace", namespace, "app", appName)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	appRef := models.NewAppRef(appName, namespace)

	appCR, err := application.Get(ctx, cluster, appRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierror.AppIsNotKnown(appName)
		}
		return apierror.InternalError(err, "failed to get the application resource")
	}

	blobUID, lookupErr := findPreviousBlobUID(appCR)
	if lookupErr != nil {
		return apierror.InternalError(lookupErr, "looking up the blob UID")
	}
	if blobUID == "" {
		return apierror.NewBadRequestError("application has no stored source")
	}

	connectionDetails, err := s3manager.GetConnectionDetails(ctx, cluster, helmchart.Namespace(), helmchart.S3ConnectionDetailsSecretName)
	if err != nil {
		return apierror.InternalError(err, "fetching the S3 connection details from the Kubernetes secret")
	}

	if validateErr := validateBlob(ctx, blobUID, appRef, connectionDetails); validateErr != nil {
		return validateErr
	}

	manager, err := s3manager.New(connectionDetails)
	if err != nil {
		return apierror.InternalError(err, "creating an S3 manager")
	}

	body, contentType, contentLength, err := manager.Open(ctx, blobUID)
	if err != nil {
		return apierror.InternalError(err, "opening the application sources blob")
	}
	defer func() {
		if closeErr := body.Close(); closeErr != nil {
			log.Errorw("failed to close source blob stream", "error", closeErr)
		}
	}()

	log.Infow("OK",
		"origin", c.Request.URL.String(),
		"returning", fmt.Sprintf("%d bytes %s", contentLength, contentType),
	)

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-source.tar"`, appName))
	c.DataFromReader(http.StatusOK, contentLength, contentType, bufio.NewReader(body), nil)
	return nil
}
