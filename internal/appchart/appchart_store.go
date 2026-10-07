package appchart

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/registry"
	"k8s.io/client-go/dynamic"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// ChartRepositoryPath is the path, below the host of Epinio's registry, under which the charts of
// custom AppCharts are stored. Each AppChart gets a repository of its own below it, named after
// the AppChart.
const ChartRepositoryPath = "epinio-charts"

// ChartStore is the registry holding the chart of one AppChart, i.e. Epinio's own registry.
//
// The chart is kept in a repository of its own. Charts of different AppCharts can therefore not
// overwrite each other. Together with HasChart, which allows to refuse a chart version which is
// already stored, this keeps the chart behind an AppChart from changing while applications are
// deployed with it.
type ChartStore interface {
	// Repository returns the OCI repository URL the chart of the AppChart is stored in. It is what
	// the AppChart references as its helm repository.
	Repository() string
	// HasChart returns true if the chart with the given name and version is stored.
	HasChart(chartName, chartVersion string) (bool, error)
	// Push stores the chart archive at the given local path.
	Push(archivePath string) error
	// Delete removes the chart with the given name and version, and anything else stored in the
	// repository of the AppChart. Removing what is not there is not an error.
	Delete(ctx context.Context, chartName, chartVersion string) error
}

// OpenChartStore returns the store for the chart of the named AppChart.
type OpenChartStore func(ctx context.Context, appChartName string) (ChartStore, error)

// InvalidChartError reports a chart archive which cannot be used as application chart.
type InvalidChartError struct {
	Reason  string
	Details string
}

func (e *InvalidChartError) Error() string {
	if e.Details == "" {
		return e.Reason
	}
	return e.Reason + ": " + e.Details
}

// AlreadyExistsError reports an AppChart of the requested name.
type AlreadyExistsError struct {
	Name string
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("appchart '%s' already exists", e.Name)
}

// ChartVersionStoredError reports a chart version which is already in the store of the AppChart.
type ChartVersionStoredError struct {
	AppChart string
	Chart    string
	Version  string
}

func (e *ChartVersionStoredError) Error() string {
	return fmt.Sprintf("chart '%s:%s' is already stored for appchart '%s'", e.Chart, e.Version, e.AppChart)
}

// LoadChartArchive loads the chart archive at the given path, and checks that it can be used as
// application chart.
func LoadChartArchive(archivePath string) (*chart.Chart, error) {
	ch, err := loader.Load(archivePath)
	if err != nil {
		return nil, &InvalidChartError{Reason: "the upload is not a valid helm chart archive", Details: err.Error()}
	}
	if err := ch.Validate(); err != nil {
		return nil, &InvalidChartError{Reason: "the helm chart is invalid", Details: err.Error()}
	}
	if ch.Metadata.Type != "" && ch.Metadata.Type != "application" {
		return nil, &InvalidChartError{
			Reason: fmt.Sprintf("chart type %q is not usable as application chart", ch.Metadata.Type),
		}
	}

	return ch, nil
}

// PushRequest is what is needed to store a chart in Epinio, and to create the AppChart for it.
type PushRequest struct {
	Name             string       // Name of the AppChart to create
	Description      string       // Optional
	ShortDescription string       // Optional
	ArchivePath      string       // Local path of the chart archive
	Chart            *chart.Chart // The chart in the archive, see LoadChartArchive
}

// Push stores the chart of the request in Epinio's registry, and creates the AppChart which
// references it.
//
// A chart version which is already stored for the AppChart is refused. The AppChart is created
// before the chart is stored. This reserves the name, and nothing is left behind in the registry
// should the creation fail. A failed push removes the AppChart again.
func Push(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	open OpenChartStore,
	req PushRequest,
) (*models.AppChartPushResponse, error) {
	exists, err := Exists(ctx, client, req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, &AlreadyExistsError{Name: req.Name}
	}

	store, err := open(ctx, req.Name)
	if err != nil {
		return nil, errors.Wrap(err, "opening the chart store")
	}

	chartName := req.Chart.Metadata.Name
	chartVersion := req.Chart.Metadata.Version

	// Refuse a chart version which is already stored. Replacing it would change the chart behind
	// deployed applications.

	stored, err := store.HasChart(chartName, chartVersion)
	if err != nil {
		return nil, err
	}
	if stored {
		return nil, &ChartVersionStoredError{AppChart: req.Name, Chart: chartName, Version: chartVersion}
	}

	helmChart := chartName + ":" + chartVersion
	helmRepo := store.Repository()

	_, err = Create(ctx, client, models.AppChartCreateRequest{
		Name:             req.Name,
		Description:      req.Description,
		ShortDescription: req.ShortDescription,
		HelmChart:        helmChart,
		HelmRepo:         helmRepo,
	})
	if err != nil {
		return nil, err
	}

	if err := store.Push(req.ArchivePath); err != nil {
		// The request context may be the reason for the failure, use one which is not canceled.
		if cleanupErr := Delete(context.WithoutCancel(ctx), client, req.Name); cleanupErr != nil {
			helpers.Logger.Errorw("removing appchart after failed push", "name", req.Name, "error", cleanupErr)
		}
		return nil, errors.Wrap(err, "pushing the chart to the registry")
	}

	return &models.AppChartPushResponse{Name: req.Name, HelmChart: helmChart, HelmRepo: helmRepo}, nil
}

// DeleteWithChart deletes the named AppChart. When its chart is stored in Epinio's registry, i.e.
// it was pushed, the chart is removed from the registry first. The AppChart is kept if that fails,
// so that the deletion can be retried instead of leaving the chart behind with nothing to
// find it by.
//
// AppCharts referencing charts elsewhere, like a url or a helm repository, are not stored by
// Epinio. Only the AppChart is deleted for them.
func DeleteWithChart(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	open OpenChartStore,
	name string,
) error {
	appChart, err := Lookup(ctx, client, name)
	if err != nil {
		return err
	}

	if appChart != nil && mayBeStored(name, appChart) {
		store, err := open(ctx, name)
		if err != nil {
			return errors.Wrap(err, "opening the chart store")
		}

		// Only the repository of Epinio's own store is touched. The same path in some other
		// registry is not ours.
		if store.Repository() == appChart.HelmRepo {
			chartName, chartVersion, _ := strings.Cut(appChart.HelmChart, ":")

			if err := store.Delete(ctx, chartName, chartVersion); err != nil {
				return errors.Wrap(err, "removing the chart from the registry")
			}
		}
	}

	return Delete(ctx, client, name)
}

// mayBeStored is true for an AppChart with a chart in an OCI repository named like the ones of
// Epinio's store. It is cheap, and avoids opening the store for all the other AppCharts.
func mayBeStored(name string, appChart *models.AppChartFull) bool {
	return registry.IsOCI(appChart.HelmRepo) &&
		strings.HasSuffix(appChart.HelmRepo, "/"+ChartRepositoryPath+"/"+name)
}
