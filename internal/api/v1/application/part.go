package application

import (
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helm"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
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

	switch partName {
	case "chart":
		return fetchAppChart(c)
	case "image":
		return fetchAppImage(c)
	case "values":
		return fetchAppValues(c, cluster, app.Meta)
	}

	return apierror.NewBadRequest("unknown part, expected chart, image, or values")
}

func fetchAppChart(c *gin.Context) apierror.APIErrors {
	// Fixed chart in the system. Request and return it.
	// Better/Faster from the local file cache ?
	// TODO: List cache directory

	return apierror.NewInternalError("disabled until app chart lookup and fetch is available")

	// response, err := http.Get(helm.StandardChart)
	// if err != nil || response.StatusCode != http.StatusOK {
	// 	c.Status(http.StatusServiceUnavailable)
	// 	return nil
	// }

	// reader := response.Body
	// contentLength := response.ContentLength
	// contentType := response.Header.Get("Content-Type")

	// requestctx.Logger(c.Request.Context()).Info("OK",
	// 	"origin", c.Request.URL.String(),
	// 	"returning", fmt.Sprintf("%d bytes %s as is", contentLength, contentType),
	// )
	// c.DataFromReader(http.StatusOK, contentLength, contentType, reader, nil)
	// return nil
}

func fetchAppImage(c *gin.Context) apierror.APIErrors {
	return apierror.NewBadRequest("image part not yet supported")
}

func fetchAppValues(c *gin.Context, cluster *kubernetes.Cluster, app models.AppRef) apierror.APIErrors {
	yaml, err := helm.Values(cluster,
		requestctx.Logger(c.Request.Context()),
		app)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKBytes(c, yaml)
	return nil
}
