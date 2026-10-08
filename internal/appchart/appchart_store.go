package appchart

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/registry"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
)

// ChartRepositoryPath is the path, below the host of Epinio's registry, under
// which the charts of custom AppCharts are stored. Each AppChart gets a
// repository of its own below it, named after the AppChart.
const ChartRepositoryPath = "epinio-charts"

// ChartStore is the registry holding the chart of one AppChart, i.e. Epinio's
// own registry.
//
// The chart is kept in a repository of its own. Charts of different AppCharts
// can therefore not overwrite each other. As the name of an AppChart is taken
// for as long as it exists, and its location cannot be changed
// (see UpdateWithChart), this keeps the chart behind an AppChart from changing
// while applications are deployed with it.
type ChartStore interface {
	// Repository returns the OCI repository URL the chart of the AppChart is i
	// stored in. It is what the AppChart references as its helm repository.
	Repository() string
	// HasChart returns true if the chart with the given name and version is
	// stored.
	HasChart(chartName, chartVersion string) (bool, error)
	// Push stores the chart archive at the given local path.
	Push(archivePath string) error
	// Delete removes the chart with the given name and version, and anything
	// else stored in the repository of the AppChart. Removing what is not there
	// is not an error.
	Delete(ctx context.Context, chartName, chartVersion string) error
}

// OpenChartStore returns the store for the chart of the named AppChart.
type OpenChartStore func(
	ctx context.Context,
	appChartName string,
) (ChartStore, error)

// BoundApps tells whether applications use the named AppChart.
type BoundApps func(ctx context.Context, appChartName string) (bool, error)

// InUseError reports an attempt to change the chart, or repository, of an
// AppChart which applications use.
type InUseError struct {
	Name string
}

func (e *InUseError) Error() string {
	return fmt.Sprintf(
		"appchart '%s' is used by applications, its chart cannot be changed",
		e.Name,
	)
}

// InvalidChartError reports a chart archive which cannot be used as
// application chart.
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

// LocationLockedError reports an attempt to change the chart, or repository,
// of an AppChart whose chart is stored in Epinio's registry.
type LocationLockedError struct {
	Name string
}

func (e *LocationLockedError) Error() string {
	return fmt.Sprintf(
		"the chart of appchart '%s' is stored by Epinio, its location cannot be changed",
		e.Name,
	)
}

// LoadChartArchive loads the chart archive at the given path, and checks that
// it can be used as application chart.
func LoadChartArchive(archivePath string) (*chart.Chart, error) {
	ch, err := loader.Load(archivePath)
	if err != nil {
		return nil, &InvalidChartError{
			Reason:  "the upload is not a valid helm chart archive",
			Details: err.Error(),
		}
	}
	if err := ch.Validate(); err != nil {
		return nil, &InvalidChartError{
			Reason:  "the helm chart is invalid",
			Details: err.Error(),
		}
	}
	if ch.Metadata.Type != "" && ch.Metadata.Type != "application" {
		return nil, &InvalidChartError{
			Reason: fmt.Sprintf(
				"chart type %q is not usable as application chart",
				ch.Metadata.Type,
			),
		}
	}

	return ch, nil
}

// PushRequest is what is needed to store a chart in Epinio, and to create the
// AppChart for it.
type PushRequest struct {
	Name             string       // Name of the AppChart to create
	Description      string       // Optional
	ShortDescription string       // Optional
	ArchivePath      string       // Local path of the chart archive
	Chart            *chart.Chart // The chart in the archive, see LoadChartArchive
}

// Push stores the chart of the request in Epinio's registry, and creates the
// AppChart which references it.
//
// The AppChart is created before the chart is stored. This reserves the name,
// and nothing is left behind in the registry should the creation fail. When
// the chart cannot be stored the AppChart is removed again.
//
// A push to the name of an existing AppChart replaces its chart, see
// replaceChart. This is how the chart of a pushed AppChart is changed, its
// location cannot be edited (see UpdateWithChart).
func Push(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	open OpenChartStore,
	boundApps BoundApps,
	req PushRequest,
) (*models.AppChartPushResponse, error) {
	appChart, err := Lookup(ctx, client, req.Name)
	if err != nil {
		return nil, err
	}
	// Without an OCI repository named like the ones of the store the name is
	// taken by a chart from elsewhere. No need to open the store to know.
	if appChart != nil && !mayBeStored(req.Name, appChart) {
		return nil, &AlreadyExistsError{Name: req.Name}
	}

	store, err := open(ctx, req.Name)
	if err != nil {
		return nil, errors.Wrap(err, "opening the chart store")
	}

	if appChart != nil {
		return replaceChart(ctx, client, store, boundApps, appChart, req)
	}

	helmChart := req.Chart.Metadata.Name + ":" + req.Chart.Metadata.Version
	helmRepo := store.Repository()

	_, err = Create(ctx, client, models.AppChartCreateRequest{
		Name:             req.Name,
		Description:      req.Description,
		ShortDescription: req.ShortDescription,
		HelmChart:        helmChart,
		HelmRepo:         helmRepo,
	})
	if err != nil {
		// Lost against a concurrent request for the same name, after the check above.
		if apierrors.IsAlreadyExists(err) {
			return nil, &AlreadyExistsError{Name: req.Name}
		}
		return nil, err
	}

	if err := storeChart(ctx, store, req); err != nil {
		// The request context may be the reason for the failure, use one which is not canceled.
		if cleanupErr := Delete(context.WithoutCancel(ctx), client, req.Name); cleanupErr != nil {
			helpers.Logger.Errorw(
				"removing appchart after failed push",
				"name",
				req.Name,
				"error",
				cleanupErr,
			)
		}
		return nil, err
	}

	return &models.AppChartPushResponse{
		Name:      req.Name,
		HelmChart: helmChart,
		HelmRepo:  helmRepo,
	}, nil
}

// replaceChart stores the chart of the request for an existing AppChart, and i
// makes the AppChart reference it. The settings and values of the AppChart are
// kept.
//
// Only the chart of an AppChart which was pushed is replaced, and only while
// no application uses it. Replacing it would change what the applications are
// deployed from with their next deployment, without them having asked for it.
//
// The new chart is stored before the AppChart is changed, and the previous
// chart is removed last. The AppChart references a chart which is in the
// registry at all times.
func replaceChart(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	store ChartStore,
	boundApps BoundApps,
	appChart *models.AppChartFull,
	req PushRequest,
) (*models.AppChartPushResponse, error) {
	// The same path in some other registry is not a chart of ours, the name is
	// simply taken.
	if store.Repository() != appChart.HelmRepo {
		return nil, &AlreadyExistsError{Name: req.Name}
	}

	inUse, err := boundApps(ctx, req.Name)
	if err != nil {
		return nil, errors.Wrap(err, "looking for applications using the appchart")
	}
	if inUse {
		return nil, &InUseError{Name: req.Name}
	}

	chartName := req.Chart.Metadata.Name
	helmChart := chartName + ":" + req.Chart.Metadata.Version

	if err := store.Push(req.ArchivePath); err != nil {
		return nil, errors.Wrap(err, "pushing the chart to the registry")
	}

	err = Update(ctx, client, req.Name, models.AppChartUpdateRequest{
		Description:      req.Description,
		ShortDescription: req.ShortDescription,
		HelmChart:        helmChart,
	})
	if err != nil {
		return nil, errors.Wrap(err, "updating the appchart")
	}

	// A chart of another name sits in a repository of its own, which the
	// deletion of the AppChart does not find anymore. Other versions of the same
	// chart go with the AppChart.
	oldName, oldVersion, _ := strings.Cut(appChart.HelmChart, ":")
	if oldName != chartName {
		if err := store.Delete(context.WithoutCancel(ctx), oldName, oldVersion); err != nil {
			helpers.Logger.Errorw(
				"removing the replaced chart from the registry",
				"appchart",
				req.Name,
				"chart",
				appChart.HelmChart,
				"error",
				err,
			)
		}
	}

	return &models.AppChartPushResponse{
		Name:      req.Name,
		HelmChart: helmChart,
		HelmRepo:  appChart.HelmRepo,
	}, nil
}

// storeChart puts the chart of the request into the store. The AppChart of the
// request has to exist already. It is what keeps other requests out of the
// repository of the AppChart.
func storeChart(ctx context.Context, store ChartStore, req PushRequest) error {
	chartName := req.Chart.Metadata.Name
	chartVersion := req.Chart.Metadata.Version

	// The AppChart was just created, so a chart found in its repository belongs
	// to no AppChart. It was left behind by an AppChart of the same name, for
	// example because the registry does not support the removal of charts.
	// Refusing the push would block the name for good, the chart is cleared out
	// instead. A registry not able to remove it has it replaced by the push.

	stored, err := store.HasChart(chartName, chartVersion)
	if err != nil {
		return err
	}
	if stored {
		helpers.Logger.Infow("replacing chart left behind in the registry",
			"appchart", req.Name, "chart", chartName, "version", chartVersion)

		if err := store.Delete(ctx, chartName, chartVersion); err != nil {
			return errors.Wrap(err, "removing the chart left behind in the registry")
		}
	}

	return errors.Wrap(
		store.Push(req.ArchivePath),
		"pushing the chart to the registry",
	)
}

// UpdateWithChart updates the named AppChart. Where its chart is found, i.e.
// the chart and the repository, can only be changed under conditions:
//
//   - Not while applications use the AppChart. It would change what they are
//     deployed from with their next deployment, without them having asked for
//     it.
//   - Not at all when the chart is stored in Epinio's registry, i.e. was
//     pushed. The stored chart would be left behind in the registry, with
//     nothing to find it by when the AppChart is deleted. Its chart is
//     replaced by pushing the new one to the same name, see Push.
//
// All other fields can always be changed.
func UpdateWithChart(
	ctx context.Context,
	client dynamic.NamespaceableResourceInterface,
	open OpenChartStore,
	boundApps BoundApps,
	name string,
	req models.AppChartUpdateRequest,
) error {
	if req.HelmChart != "" || req.HelmRepo != "" {
		appChart, err := Lookup(ctx, client, name)
		if err != nil {
			return err
		}

		moved := appChart != nil &&
			((req.HelmChart != "" && req.HelmChart != appChart.HelmChart) ||
				(req.HelmRepo != "" && req.HelmRepo != appChart.HelmRepo))

		if moved {
			inUse, err := boundApps(ctx, name)
			if err != nil {
				return errors.Wrap(err, "looking for applications using the appchart")
			}
			if inUse {
				return &InUseError{Name: name}
			}
		}

		if moved && mayBeStored(name, appChart) {
			store, err := open(ctx, name)
			if err != nil {
				return errors.Wrap(err, "opening the chart store")
			}
			if store.Repository() == appChart.HelmRepo {
				return &LocationLockedError{Name: name}
			}
		}
	}

	return Update(ctx, client, name, req)
}

// DeleteWithChart deletes the named AppChart. When its chart is stored in
// Epinio's registry, i.e. it was pushed, the chart is removed from the
// registry first. The AppChart is kept if that fails, so that the deletion can
// be retried instead of leaving the chart behind with nothing to find it by.
//
// AppCharts referencing charts elsewhere, like a url or a helm repository,
// are not stored by Epinio. Only the AppChart is deleted for them.
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

		// Only the repository of Epinio's own store is touched. The same path in
		// some other registry is not ours.
		if store.Repository() == appChart.HelmRepo {
			chartName, chartVersion, _ := strings.Cut(appChart.HelmChart, ":")

			if err := store.Delete(ctx, chartName, chartVersion); err != nil {
				return errors.Wrap(err, "removing the chart from the registry")
			}
		}
	}

	return Delete(ctx, client, name)
}

// mayBeStored is true for an AppChart with a chart in an OCI repository named
// like the ones of Epinio's store. It is cheap, and avoids opening the store
// for all the other AppCharts.
func mayBeStored(name string, appChart *models.AppChartFull) bool {
	return registry.IsOCI(appChart.HelmRepo) &&
		strings.HasSuffix(appChart.HelmRepo, "/"+ChartRepositoryPath+"/"+name)
}
