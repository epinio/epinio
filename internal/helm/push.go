package helm

import (
	"context"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	epinioregistry "github.com/epinio/epinio/internal/registry"
)

// ChartRepositoryPath is the path, below the host of Epinio's registry, under which custom
// AppCharts are stored.
const ChartRepositoryPath = "epinio-charts"

// PushChartToEpinioRegistry pushes the chart archive at the given local path into Epinio's own
// registry, under ChartRepositoryPath, and returns the OCI repository URL the chart was pushed to.
// The chart is then referenced by an AppChart through this URL and the chart's name and version.
//
// The server logs into the registry with the registry credentials it already uses for application
// images. The user never sees, or has to supply, credentials.
func PushChartToEpinioRegistry(ctx context.Context, cluster *kubernetes.Cluster, archivePath string) (string, error) {
	connectionDetails, err := epinioregistry.GetConnectionDetails(
		ctx, cluster, helmchart.Namespace(), epinioregistry.CredentialsSecretName)
	if err != nil {
		return "", errors.Wrap(err, "getting the registry connection details")
	}

	// The public URL is the one the server itself is able to reach, and the one helm resolves
	// charts through, see getChartReference.
	registryURL, err := connectionDetails.PublicRegistryURL()
	if err != nil {
		return "", errors.Wrap(err, "determining the registry url")
	}
	if registryURL == "" {
		return "", errors.New("no registry url found in the registry credentials")
	}

	hostname := ociHostname(registryURL)

	creds := registryLoginFor(connectionDetails, hostname)
	if creds == nil {
		return "", errors.New("no credentials found for registry " + hostname)
	}

	client, err := GetHelmClient(cluster.RestConfig, "")
	if err != nil {
		return "", errors.Wrap(err, "create a helm client")
	}

	err = client.RegistryLogin(creds.Hostname, creds.Username, creds.Password, action.WithInsecure(creds.Insecure))
	if err != nil {
		return "", errors.Wrap(err, "logging into the registry")
	}

	repository := "oci://" + hostname + "/" + ChartRepositoryPath

	_, err = client.Push(archivePath, repository, action.WithInsecureSkipTLSVerify(creds.Insecure))
	if err != nil {
		return "", errors.Wrap(err, "pushing the chart")
	}

	return repository, nil
}
