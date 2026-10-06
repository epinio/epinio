package helm

import (
	"context"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	epinioregistry "github.com/epinio/epinio/internal/registry"
)

// ChartRepositoryPath is the path, below the host of Epinio's registry, under which the charts of
// custom AppCharts are stored. Each AppChart gets a repository of its own below it, named after
// the AppChart, see ChartRegistry.
const ChartRepositoryPath = "epinio-charts"

// ChartRegistry is Epinio's own registry, as target for the push of the chart of one AppChart.
//
// The chart goes into a repository of its own, `<registry>/epinio-charts/<appchart name>`. Charts
// of different AppCharts can therefore not overwrite each other. Together with HasChart, which
// allows to refuse a chart version which is already stored, this keeps the chart behind an
// AppChart from changing while applications are deployed with it.
type ChartRegistry struct {
	client     *SynchronizedClient
	creds      *registryLogin
	repository string
}

// OpenChartRegistry returns Epinio's registry, set up for the chart of the named AppChart.
//
// The server logs into the registry with the registry credentials it already uses for application
// images. The user never sees, or has to supply, credentials.
func OpenChartRegistry(ctx context.Context, cluster *kubernetes.Cluster, appChartName string) (*ChartRegistry, error) {
	connectionDetails, err := epinioregistry.GetConnectionDetails(
		ctx, cluster, helmchart.Namespace(), epinioregistry.CredentialsSecretName)
	if err != nil {
		return nil, errors.Wrap(err, "getting the registry connection details")
	}

	// The public URL is the one the server itself is able to reach, and the one helm resolves
	// charts through, see getChartReference.
	registryURL, err := connectionDetails.PublicRegistryURL()
	if err != nil {
		return nil, errors.Wrap(err, "determining the registry url")
	}
	if registryURL == "" {
		return nil, errors.New("no registry url found in the registry credentials")
	}

	hostname := ociHostname(registryURL)

	creds := registryLoginFor(connectionDetails, hostname)
	if creds == nil {
		return nil, errors.New("no credentials found for registry " + hostname)
	}

	client, err := GetHelmClient(cluster.RestConfig, "")
	if err != nil {
		return nil, errors.Wrap(err, "create a helm client")
	}

	err = client.RegistryLogin(creds.Hostname, creds.Username, creds.Password, action.WithInsecure(creds.Insecure))
	if err != nil {
		return nil, errors.Wrap(err, "logging into the registry")
	}

	return &ChartRegistry{
		client:     client,
		creds:      creds,
		repository: "oci://" + hostname + "/" + ChartRepositoryPath + "/" + appChartName,
	}, nil
}

// Repository returns the OCI repository URL the chart of the AppChart is stored in. It is what an
// AppChart references as its helm repository. The chart is then referenced through its name and
// version.
func (r *ChartRegistry) Repository() string {
	return r.repository
}

// HasChart returns true if the registry already holds the chart with the given name and version
// in the repository of the AppChart.
func (r *ChartRegistry) HasChart(chartName, chartVersion string) (bool, error) {
	exists, err := r.client.OCIChartExists(ociChartRef(r.repository, chartName), chartVersion)
	return exists, errors.Wrap(err, "checking for the chart in the registry")
}

// Push pushes the chart archive at the given local path into the repository of the AppChart.
// It does not check for an existing chart of the same name and version, see HasChart.
func (r *ChartRegistry) Push(archivePath string) error {
	_, err := r.client.Push(archivePath, r.repository, action.WithInsecureSkipTLSVerify(r.creds.Insecure))
	return errors.Wrap(err, "pushing the chart")
}
