package appchart

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helm"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/validation"
)

// maxChartArchiveSize is the maximum size of a chart archive accepted for a push.
const maxChartArchiveSize = 32 << 20 // 32 MiB

// Push handles the API endpoint POST /appcharts/push.
//
// It receives a helm chart archive (multipart field `file`) and the name for
// the new application chart (field `name`, optional fields `description` and
// `short_description`). The chart is pushed to Epinio's own OCI registry, and
// an application chart referencing it is created.
//
// For the name of an application chart which was pushed before, and which no
// application uses, the chart of that application chart is replaced.
func Push(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	log.Infow("push appchart")
	defer log.Infow("return")

	// Grab file from the request, and limit the size of the request body to
	// avoid attacks.
	c.Request.Body = http.MaxBytesReader(
		c.Writer,
		c.Request.Body,
		maxChartArchiveSize,
	)

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		return apierror.NewBadRequestError(err.Error()).
			WithDetails("can't read multipart file input")
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Errorw("file failed to close", "error", err)
		}
	}()

	// Validate name of the application chart. It is required, and must be a
	// valid DNS subdomain name.
	name := c.Request.FormValue("name")
	if name == "" {
		return apierror.NewBadRequestError("name is required")
	}
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return apierror.NewBadRequestErrorf(
			"invalid application chart name %q: %s",
			name,
			errs[0],
		)
	}

	// Save the upload to a local file. helm pushes from a file, and loading it
	// from there also validates the archive before the cluster is touched, or
	// anything is written to the registry.
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

	chart, err := appchart.LoadChartArchive(archive.Name())
	if err != nil {
		return pushError(err)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	client, err := cluster.ClientAppChart()
	if err != nil {
		return apierror.InternalError(err)
	}

	log.Infow(
		"push chart",
		"name",
		name,
		"chart",
		chart.Metadata.Name,
		"version",
		chart.Metadata.Version,
	)

	result, err := appchart.Push(
		ctx,
		client,
		chartStoreOpener(cluster),
		boundAppsOf(cluster),
		appchart.PushRequest{
			Name:             name,
			Description:      c.Request.FormValue("description"),
			ShortDescription: c.Request.FormValue("short_description"),
			ArchivePath:      archive.Name(),
			Chart:            chart,
		},
	)
	if err != nil {
		return pushError(err)
	}

	log.Infow("appchart created", "name", name)
	response.OKReturn(c, result)
	return nil
}

// chartStoreOpener returns the function opening Epinio's registry as store for
// charts.
func chartStoreOpener(cluster *kubernetes.Cluster) appchart.OpenChartStore {
	return func(
		ctx context.Context,
		appChartName string,
	) (appchart.ChartStore, error) {
		return helm.OpenChartRegistry(ctx, cluster, appChartName)
	}
}

// boundAppsOf returns the function telling whether applications use an
// application chart.
func boundAppsOf(cluster *kubernetes.Cluster) appchart.BoundApps {
	return func(ctx context.Context, appChartName string) (bool, error) {
		inUse, err := application.ChartsInUse(ctx, cluster)
		return inUse[appChartName], err
	}
}

// pushError maps the errors of the push of a chart to API errors.
func pushError(err error) apierror.APIErrors {
	var invalid *appchart.InvalidChartError
	var exists *appchart.AlreadyExistsError
	var inUse *appchart.InUseError

	switch {
	case errors.As(err, &invalid):
		bad := apierror.NewBadRequestError(invalid.Reason)
		if invalid.Details != "" {
			bad = bad.WithDetails(invalid.Details)
		}
		return bad
	case errors.As(err, &exists):
		return apierror.AppChartAlreadyKnown(exists.Name)
	case errors.As(err, &inUse):
		return inUseError(inUse)
	default:
		return apierror.InternalError(err)
	}
}

// inUseError is the API error for a change to the chart of an application
// chart applications use.
func inUseError(err *appchart.InUseError) apierror.APIErrors {
	return apierror.NewAPIError(
		fmt.Sprintf(
			"application chart '%s' is used by applications, its chart cannot be changed",
			err.Name,
		),
		http.StatusConflict,
	)
}
