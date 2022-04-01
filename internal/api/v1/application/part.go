package application

import (
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helm"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
)

// GetPart handles the API endpoint GET /namespaces/:namespace/applications/:app/part/:part
// It determines the contents of the requested part (values, chart, image) and returns as
// the response of the handler.
func (hc Controller) GetPart(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	partName := c.Param("part")

	if partName != "values" && partName != "chart" && partName != "image" {
		return apierror.NewBadRequest("unknown part, expected chart, image, or values")
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	if err := hc.validateNamespace(ctx, cluster, namespace); err != nil {
		return err
	}

	app, err := application.Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return apierror.InternalError(err)
	}

	if app == nil {
		return apierror.AppIsNotKnown(appName)
	}

	if app.Workload == nil {
		// While the app exists it has no workload, and therefore no chart to export
		return apierror.NewBadRequest("No chart available for application without workload")
	}

	if partName == "chart" {
		// Fixed chart in the system. Request and return it.
		// Better/Faster from the local file cache ?
		// TODO: List cache directory

		response, err := http.Get(helm.StandardChart)
		if err != nil || response.StatusCode != http.StatusOK {
			c.Status(http.StatusServiceUnavailable)
			return nil
		}

		reader := response.Body
		contentLength := response.ContentLength
		contentType := response.Header.Get("Content-Type")

		requestctx.Logger(c.Request.Context()).Info("OK",
			"origin", c.Request.URL.String(),
			"returning", fmt.Sprintf("%d bytes %s as is", contentLength, contentType),
		)
		c.DataFromReader(http.StatusOK, contentLength, contentType, reader, nil)
		return nil
	}

	if partName == "image" {
		return apierror.NewInternalError("image part not yet supported")
	}

	if partName == "values" {
		log := requestctx.Logger(ctx)

		yaml, err := helm.Values(cluster, log, app.Meta)

		if err != nil {
			return apierror.InternalError(err)
		}

		response.OKBytes(c, yaml)
		return nil
	}

	return apierror.NewBadRequest("unknown part, expected chart, image, or values")
}
