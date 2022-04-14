package application

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

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
	"gopkg.in/yaml.v2"
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

	chartArchive := appChart.HelmChart

	if appChart.HelmRepo != "" {
		// The chart is specified as simple name, and resolved to actual archive through a helm repo

		// This code is a __HACK__. At various levels.
		//
		// - We create and initialize a mittwald client, this gives us the basic
		//   dir structure needed.
		//
		// - We add the repository needed. This gives us the chart and index files
		//   for it in the above directory hierarchy.
		//
		// - We do __NOT__ use a low-level helm puller action. Even setting it up
		//   with configuration and settings of the above client it will look in
		//   the wrong place for the repo index. I.e. looks to completely ignore
		//   the RepositoryCache setting.
		//
		// - So, to continue the hack, we access the repo index.yaml directly,
		//   i.e. read in, unmarshal into minimally required structure and then
		//   locate the chart and its urls.
		//
		// The advantage of this hack: We get a fetchable url we can feed into the
		// part invoked when the chart was specified as direct url. No going
		// through a temp file.

		// Split chart ref into name and version

		helmChart := appChart.HelmChart
		helmVersion := ""

		pieces := strings.SplitN(helmChart, ":", 2)
		if len(pieces) == 2 {
			helmVersion = pieces[1]
			helmChart = pieces[0]
		}

		// Set up client and repo, ensures proper directory structure and presence of index file

		name := names.GenerateResourceName("hr-" + base64.StdEncoding.EncodeToString([]byte(appChart.HelmRepo)))

		client, err := helm.GetHelmClient(cluster.RestConfig, logger, app.Namespace)
		if err != nil {
			return apierror.InternalError(err)
		}

		if err := client.AddOrUpdateChartRepo(repo.Entry{
			Name: name,
			URL:  appChart.HelmRepo,
		}); err != nil {
			return apierror.InternalError(err)
		}

		// Read index into memory

		content, err := ioutil.ReadFile("/tmp/.helmcache/" + name + "-index.yaml")
		if err != nil {
			return apierror.InternalError(err)
		}

		// Get minimal structure need to locate the chart by name, and version.

		var index struct {
			Entries map[string][]struct {
				Version string   `yaml:"version"`
				URLs    []string `yaml:"urls"`
			} `yaml:"entries"`
		}

		err = yaml.Unmarshal(content, &index)
		if err != nil {
			return apierror.InternalError(err)
		}

		entries, ok := index.Entries[helmChart]
		if !ok {
			return apierror.NewBadRequest("Chart '" + helmChart + "' not found")
		}

		found := false
		for _, entry := range entries {
			if helmVersion == "" {
				// No version specified. Take first found.  Assumes that
				// first in sequence is highest version, i.e. latest.
				found = true
				chartArchive = entry.URLs[0]
				break
			}
			if entry.Version == helmVersion {
				found = true
				chartArchive = entry.URLs[0]
				break
			}
		}
		if !found {
			return apierror.NewBadRequest("Chart '" + helmChart + "' version '" + helmVersion + "' not found")
		}

		// Fall through to fetching the chart by url
	}

	// Chart is (now) specified as direct url to the tarball

	response, err := http.Get(chartArchive) // nolint:gosec // app chart repo ref
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

//
// SCRATCH AREA ___________________________________________________________________________________
//

// Initial fetchChart code for helm repo + name - attempt to create a low-level puller using the mittwald client information ...

// When using the repo name for the url we get a `could not find protocol handler`.
// When using the actual url it looks in the wrong place for the for index.yaml (/.cache), and it encodes the name itself from the repoURL, different from us.

// ==> Hence the HACK of going directly for the index.yaml, unmarshalling, and searching inside of it.

// // Chart is specified as simple name, and resolved to actual archive through a helm repo

// client, err := helm.GetHelmClient(cluster.RestConfig, logger, app.Namespace)
// if err != nil {
// 	return apierror.InternalError(err)
// }

// name := names.GenerateResourceName("hr-" + base64.StdEncoding.EncodeToString([]byte(appChart.HelmRepo)))

// if err := client.AddOrUpdateChartRepo(repo.Entry{
// 	Name: name,
// 	URL:  appChart.HelmRepo,
// }); err != nil {
// 	return apierror.InternalError(err)
// }

// // Compute chart name and version - enable when we have fetch

// helmChart := appChart.HelmChart
// helmVersion := ""

// pieces := strings.SplitN(helmChart, ":", 2)
// if len(pieces) == 2 {
// 	helmVersion = pieces[1]
// 	helmChart = pieces[0]
// }

// helmChart = fmt.Sprintf("%s/%s", name, helmChart)

// // TODO: Fetch chart tarball from repo, via helm client
// // BAD: Mittwald client used here does not seem to support such.

// // HACK (working with the low-level puller ran into issues - something with protocol handler ?)

// puller := action.NewPullWithOpts(action.WithConfig(client.(*hc.HelmClient).ActionConfig))
// puller.Settings = client.(*hc.HelmClient).Settings
// puller.RepoURL = name //appChart.HelmRepo
// puller.Version = helmVersion
// puller.DestDir = "/tmp"

// fmt.Printf("ZZZ\n")
// fmt.Printf("ZZZ P ((%+v))\n", puller)
// fmt.Printf("ZZZ\n")
// fmt.Printf("ZZZ S ((%+v))\n", puller.Settings)
// fmt.Printf("ZZZ\n")
// fmt.Printf("ZZZ C ((%+v))\n", client.(*hc.HelmClient))

// // walk the pod directory hierarchy ...
// fmt.Printf("ZZZ\n")
// err = filepath.Walk("/tmp",
// 	func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			fmt.Printf("ZZZ E %+v\n", err)
// 			return nil
// 		}
// 		fmt.Printf("ZZZ W %s (%+v)\n", path, info.Size())
// 		return nil
// 	})
// if err != nil {
// 	fmt.Printf("ZZZ E %+v\n", err)
// }

// fmt.Printf("ZZZ\n")
// fmt.Printf("ZZZ charts.txt\n")
// fmt.Printf("ZZZ\n")

// content, err := ioutil.ReadFile("/tmp/.helmcache/" + name + "-charts.txt")
// if err != nil {
// 	fmt.Printf("ZZZ E %+v\n", err)
// } else {
// 	fmt.Printf("ZZZ %s\n", string(content))
// }

// fmt.Printf("ZZZ\n")
// fmt.Printf("ZZZ index.yaml\n")

// content, err = ioutil.ReadFile("/tmp/.helmcache/" + name + "-index.yaml")
// if err != nil {
// 	fmt.Printf("ZZZ E %+v\n", err)
// } else {
// 	fmt.Printf("ZZZ %s\n", string(content))
// }
// fmt.Printf("ZZZ\n")
// fmt.Printf("ZZZ\n")
// fmt.Printf("ZZZ\n")
// fmt.Printf("ZZZ\n")

// out, err := puller.Run(helmChart)
// // Currently fails to access index.yaml for repo - wromng directory
// // /.cache/helm/repository/3ExWe8EM2EGE91vV94Lr3vv8yxg=-index.yaml
// // config is /tmp.helmcache

// fmt.Printf("ZZZ\n")
// fmt.Printf("ZZZ\n")
// fmt.Printf("ZZZ\n")

// // read index.yaml, locate chart by name and version, extract archive link, and fetch ...

// if err != nil {
// 	return apierror.InternalError(err)
// }

// fmt.Printf("XXX\nXXX %s\nXXX\n", out)

// return apierror.NewInternalError("unable to fetch chart tarball from helm repo")

// --------------------------------------------------------------------------------

// // walk the pod directory hierarchy ...
// fmt.Printf("ZZZ\n")
// err = filepath.Walk("/tmp",
// 	func(path string, info os.FileInfo, err error) error {
// 		if err != nil {
// 			fmt.Printf("ZZZ E %+v\n", err)
// 			return nil
// 		}
// 		fmt.Printf("ZZZ W %s (%+v)\n", path, info.Size())
// 		return nil
// 	})
// if err != nil {
// 	fmt.Printf("ZZZ E %+v\n", err)
// }
// fmt.Printf("ZZZ\n")
// fmt.Printf("ZZZ %T %+v\n", index, index)
