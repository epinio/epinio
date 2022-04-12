package application

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helm"
	"github.com/epinio/epinio/internal/names"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/repo"
)

// GetPart handles the API endpoint GET /namespaces/:namespace/applications/:app/part/:part
// It determines the contents of the requested part (values, chart, image) and returns as
// the response of the handler.
func (hc Controller) GetPart(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	appName := c.Param("app")
	partName := c.Param("part")
	logger := requestctx.Logger(ctx)

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
		return fetchAppChart(c, ctx, logger, cluster, app.Meta)
	case "image":
		return fetchAppImage(c)
	case "values":
		return fetchAppValues(c, logger, cluster, app.Meta)
	}

	return apierror.NewBadRequest("unknown part, expected chart, image, or values")
}

func fetchAppChart(c *gin.Context, ctx context.Context, logger logr.Logger, cluster *kubernetes.Cluster, app models.AppRef) apierror.APIErrors {

	// Get application
	theApp, err := application.Lookup(ctx, cluster, app.Namespace, app.Name)
	if err != nil {
		return apierror.InternalError(err)
	}

	if theApp == nil {
		return apierror.AppIsNotKnown(app.Name)
	}

	// Get the application's app chart
	appChart, err := appchart.Lookup(ctx, cluster, theApp.Configuration.AppChart)
	if err != nil {
		return apierror.InternalError(err)
	}
	if appChart == nil {
		return apierror.AppChartIsNotKnown(theApp.Configuration.AppChart)
	}

	if appChart.HelmRepo != "" {
		// Chart is specified as simple name, and resolved through a helm repo

		client, err := helm.GetHelmClient(cluster, logger, app.Namespace)
		if err != nil {
			return apierror.InternalError(err)
		}

		name := names.GenerateResourceName("hr-" + base64.StdEncoding.EncodeToString([]byte(appChart.HelmRepo)))

		if err := client.AddOrUpdateChartRepo(repo.Entry{
			Name: name,
			URL:  appChart.HelmRepo,
		}); err != nil {
			return apierror.InternalError(err)
		}

		// Compute chart name and version - enable when we have fetch
		//
		// helmChart := appChart.HelmChart
		// helmVersion := ""
		//
		// pieces := strings.SplitN(helmChart, ":", 2)
		// if len(pieces) == 2 {
		// 	helmVersion = pieces[1]
		// 	helmChart = pieces[0]
		// }
		//
		// helmChart = fmt.Sprintf("%s/%s", name, helmChart)
		//
		// TODO: Fetch chart tarball from repo, via helm client
		// BAD: Mittwald client used here does not seem to support such.

		return apierror.NewInternalError("unable to fetch chart tarball from helm repo")
	}

	// Chart is specified as direct url to the tarball

	response, err := http.Get(appChart.HelmChart)
	if err != nil || response.StatusCode != http.StatusOK {
		c.Status(http.StatusServiceUnavailable)
		return nil
	}

	reader := response.Body
	contentLength := response.ContentLength
	contentType := response.Header.Get("Content-Type")

	logger.Info("OK",
		"origin", c.Request.URL.String(),
		"returning", fmt.Sprintf("%d bytes %s as is", contentLength, contentType),
	)
	c.DataFromReader(http.StatusOK, contentLength, contentType, reader, nil)
	return nil
}

func fetchAppImage(c *gin.Context) apierror.APIErrors {
	return apierror.NewBadRequest("image part not yet supported")
}

func fetchAppValues(c *gin.Context, logger logr.Logger, cluster *kubernetes.Cluster, app models.AppRef) apierror.APIErrors {
	yaml, err := helm.Values(cluster, logger, app)
	if err != nil {
		return apierror.InternalError(err)
	}

	response.OKBytes(c, yaml)
	return nil
}
