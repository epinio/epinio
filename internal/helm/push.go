package helm

import (
	"context"
	"os"
	"strings"

	"github.com/pkg/errors"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/helmchart"
	epinioregistry "github.com/epinio/epinio/internal/registry"
)

// ChartRegistry is Epinio's own registry, as store for the chart of one
// AppChart. It implements appchart.ChartStore.
//
// The chart goes into a repository of its own,
// `<registry>/<namespace>/epinio_appchart_<name>`, next to the application
// images. See appchart.ChartNameFor.
type ChartRegistry struct {
	client     *SynchronizedClient
	creds      *registryLogin
	repository string
	chartName  string
}

// OpenChartRegistry returns Epinio's registry, set up for the chart of the
// named AppChart.
//
// The server logs into the registry with the registry credentials it already
// uses for application images. The user never sees, or has to supply,
// credentials.
func OpenChartRegistry(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	appChartName string,
) (*ChartRegistry, error) {
	connectionDetails, err := epinioregistry.GetConnectionDetails(
		ctx, cluster, helmchart.Namespace(), epinioregistry.CredentialsSecretName)
	if err != nil {
		return nil, errors.Wrap(err, "getting the registry connection details")
	}

	// The same location the application images are pushed to. Pushing anywhere
	// else would need rights on the registry which Epinio may not have, like a
	// second namespace on Docker Hub.
	base, err := connectionDetails.RepositoryBase()
	if err != nil {
		return nil, errors.Wrap(err, "determining the registry url")
	}

	hostname := ociHostname(base)

	creds := registryLoginFor(connectionDetails, hostname)
	if creds == nil {
		return nil, errors.New("no credentials found for registry " + hostname)
	}
	creds.RootCAs, err = registryRootCAs(ctx, cluster.RestConfig)
	if err != nil {
		return nil, err
	}

	client, err := GetHelmClient(cluster.RestConfig, "")
	if err != nil {
		return nil, errors.Wrap(err, "create a helm client")
	}

	err = client.RegistryLogin(
		creds.Hostname,
		creds.Username,
		creds.Password,
		creds.Insecure,
	)
	if err != nil {
		return nil, errors.Wrap(err, "logging into the registry")
	}

	return &ChartRegistry{
		client:     client,
		creds:      creds,
		repository: "oci://" + ociLocation(base),
		chartName:  appchart.ChartNameFor(appChartName),
	}, nil
}

// Repository returns the OCI URL below which the chart of the AppChart is
// stored. It is what an AppChart references as its helm repository.
func (r *ChartRegistry) Repository() string {
	return r.repository
}

// ChartName returns the name the chart is stored under, below Repository. It is
// what an AppChart references as its chart, together with the version.
func (r *ChartRegistry) ChartName() string {
	return r.chartName
}

// HasChart returns true if the registry already holds the chart with the given
// version in the repository of the AppChart.
func (r *ChartRegistry) HasChart(chartVersion string) (bool, error) {
	exists, err := r.client.OCIChartExists(r.chartRef(), chartVersion)
	return exists, errors.Wrap(err, "checking for the chart in the registry")
}

// Push pushes the chart archive at the given local path into the repository of
// the AppChart, as the given version. It does not check for an existing chart
// of the same version, see HasChart.
//
// The chart is stored under the name of the repository, not its own name. helm
// itself would push it to `<repository>/<chart name>`, which neither keeps the
// charts of different AppCharts apart, nor is a valid repository on registries
// like Docker Hub, see appchart.ChartNameFor.
func (r *ChartRegistry) Push(archivePath, chartVersion string) error {
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return errors.Wrap(err, "reading the chart archive")
	}

	err = r.client.PushAt(data, r.imageRef(chartVersion))
	return errors.Wrap(err, "pushing the chart")
}

// Delete removes the chart with the given version from the repository of the
// AppChart, together with all other tags of the repository. A chart which is
// not in the registry is not an error.
func (r *ChartRegistry) Delete(ctx context.Context, chartVersion string) error {
	// Like for the push, the certificate of a registry inside the cluster is
	// not verified, and a private CA is trusted.
	tlsConfig := r.creds.tlsConfig()

	creds := epinioregistry.RegistryCredentials{
		URL:      r.creds.URL,
		Username: r.creds.Username,
		Password: r.creds.Password,
	}
	err := epinioregistry.DeleteImage(
		ctx,
		r.imageRef(chartVersion),
		creds,
		tlsConfig,
	)

	return errors.Wrap(err, "deleting the chart")
}

// chartRef returns the OCI reference of the chart, without version
// (oci://host/path/name).
func (r *ChartRegistry) chartRef() string {
	return ociChartRef(r.repository, r.chartName)
}

// imageRef returns the reference of the given version of the chart, the way
// registry clients take it (host/path/name:tag).
func (r *ChartRegistry) imageRef(chartVersion string) string {
	return strings.TrimPrefix(r.chartRef(), "oci://") +
		":" + ociTag(chartVersion)
}
