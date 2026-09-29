package appchart

import (
	"io"
	"net/http"
	"os"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helm"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	models "github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/util/validation"
)

// maxChartArchiveSize is the maximum size of a chart archive accepted for a push.
const maxChartArchiveSize = 32 << 20 // 32 MiB

// Push handles the API endpoint POST /appcharts/push.
//
// It receives a helm chart archive (multipart field `file`) and the name for the new application
// chart (field `name`, optional fields `description` and `short_description`). The chart is
// pushed to Epinio's own OCI registry, and an application chart referencing it is created.
func Push(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	log.Infow("push appchart")
	defer log.Infow("return")

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxChartArchiveSize)

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		return apierror.NewBadRequestError(err.Error()).WithDetails("can't read multipart file input")
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Errorw("file failed to close", "error", err)
		}
	}()

	name := c.Request.FormValue("name")
	if name == "" {
		return apierror.NewBadRequestError("name is required")
	}
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return apierror.NewBadRequestErrorf("invalid application chart name %q: %s", name, errs[0])
	}

	// Save the upload to a local file. helm pushes from a file, and loading it from there also
	// validates the archive before the cluster is touched, or anything is written to the registry.

	archive, err := os.CreateTemp("", "epinio-chart-push-*.tgz")
	if err != nil {
		return apierror.InternalError(err, "creating a temporary file for the chart")
	}
	defer func() { _ = os.Remove(archive.Name()) }()

	_, err = io.Copy(archive, file)
	closeErr := archive.Close()
	if err != nil {
		return apierror.InternalError(err, "saving the chart archive")
	}
	if closeErr != nil {
		return apierror.InternalError(closeErr, "saving the chart archive")
	}

	chart, err := loader.Load(archive.Name())
	if err != nil {
		return apierror.NewBadRequestError(err.Error()).WithDetails("the upload is not a valid helm chart archive")
	}
	if err := chart.Validate(); err != nil {
		return apierror.NewBadRequestError(err.Error()).WithDetails("the helm chart is invalid")
	}
	if chart.Metadata.Type != "" && chart.Metadata.Type != "application" {
		return apierror.NewBadRequestErrorf("chart type %q is not usable as application chart", chart.Metadata.Type)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	client, err := cluster.ClientAppChart()
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Infow("check existence", "name", name)
	exists, err := appchart.Exists(ctx, client, name)
	if err != nil {
		return apierror.InternalError(err)
	}
	if exists {
		return apierror.AppChartAlreadyKnown(name)
	}

	log.Infow("push chart", "chart", chart.Metadata.Name, "version", chart.Metadata.Version)

	repository, err := helm.PushChartToEpinioRegistry(ctx, cluster, archive.Name())
	if err != nil {
		return apierror.InternalError(errors.Wrap(err, "pushing the chart to the registry"))
	}

	helmChart := chart.Metadata.Name + ":" + chart.Metadata.Version

	log.Infow("create appchart resource", "name", name, "helmChart", helmChart, "helmRepo", repository)
	_, err = appchart.Create(ctx, client, models.AppChartCreateRequest{
		Name:             name,
		Description:      c.Request.FormValue("description"),
		ShortDescription: c.Request.FormValue("short_description"),
		HelmChart:        helmChart,
		HelmRepo:         repository,
	})
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Infow("appchart created", "name", name)
	response.OKReturn(c, models.AppChartPushResponse{
		Name:      name,
		HelmChart: helmChart,
		HelmRepo:  repository,
	})
	return nil
}
