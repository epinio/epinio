package helm

import (
	"context"
	"crypto/tls"
	"strings"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/helmchart"
	epinioregistry "github.com/epinio/epinio/internal/registry"
)

// ChartRegistry is Epinio's own registry, as store for the chart of one AppChart. It implements
// appchart.ChartStore.
//
// The chart goes into a repository of its own, `<registry>/epinio-charts/<appchart name>`, see
// appchart.ChartRepositoryPath.
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
		repository: "oci://" + hostname + "/" + appchart.ChartRepositoryPath + "/" + appChartName,
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

// Delete removes the chart with the given name and version from the repository of the AppChart,
// together with all other tags of the repository. A chart which is not in the registry is not an
// error.
func (r *ChartRegistry) Delete(ctx context.Context, chartName, chartVersion string) error {
	imageURL := strings.TrimPrefix(ociChartRef(r.repository, chartName), "oci://") + ":" + ociTag(chartVersion)

	// Like for the push, the certificate of a registry inside the cluster is not verified.
	var tlsConfig *tls.Config
	if r.creds.Insecure {
		tlsConfig = &tls.Config{
			InsecureSkipVerify: true, // nolint:gosec // registry in the cluster, with a self-signed certificate
			MinVersion:         tls.VersionTLS12,
		}
	}

	err := epinioregistry.DeleteImage(ctx, imageURL, epinioregistry.RegistryCredentials{
		URL:      r.creds.URL,
		Username: r.creds.Username,
		Password: r.creds.Password,
	}, tlsConfig)

	return errors.Wrap(err, "deleting the chart")
}
