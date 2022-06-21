// Package helm contains the epinio-specific core to the helm client libraries. It exposes
// the functionality to deploy and remove helm charts/releases.
package helm

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/routes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/go-logr/logr"
	hc "github.com/mittwald/go-helm-client"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	helmrelease "helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/client-go/rest"
)

type ChartParameters struct {
	models.AppRef                        // Application: name & namespace
	Context        context.Context       // Operation context
	Cluster        *kubernetes.Cluster   // Cluster to talk to.
	Chart          string                // Name of Chart CR to use for deployment
	ImageURL       string                // Application Image
	Username       string                // User causing the (re)deployment
	Instances      int32                 // Number Of Desired Replicas
	StageID        string                // Stage ID that produced ImageURL
	Environment    models.EnvVariableMap // App Environment
	Configurations []string              // Bound Configurations (list of names)
	Routes         []string              // Desired application routes
	Start          *int64                // Nano-epoch of deployment. Optional. Used to force a restart, even when nothing else has changed.
}

func Values(cluster *kubernetes.Cluster, logger logr.Logger, app models.AppRef) ([]byte, error) {
	none := []byte{}

	client, err := GetHelmClient(cluster.RestConfig, logger, app.Namespace)
	if err != nil {
		return none, err
	}

	values, err := client.GetReleaseValues(names.ReleaseName(app.Name), false)
	if err != nil {
		return none, err
	}

	yaml, err := yaml.Marshal(values)
	if err != nil {
		return none, err
	}

	return yaml, nil
}

func Remove(cluster *kubernetes.Cluster, logger logr.Logger, app models.AppRef) error {
	client, err := GetHelmClient(cluster.RestConfig, logger, app.Namespace)
	if err != nil {
		return err
	}

	return client.UninstallReleaseByName(names.ReleaseName(app.Name))
}

func Deploy(logger logr.Logger, parameters ChartParameters) error {
	// Find the app chart to use for the deployment.
	appChart, err := appchart.Lookup(parameters.Context, parameters.Cluster, parameters.Chart)
	if err != nil {
		return errors.Wrap(err, "looking up application chart")
	}
	if appChart == nil {
		return fmt.Errorf("Unable to deploy, chart %s not found", parameters.Chart)
	}

	// YAML string - TODO ? Use unstructured as intermediary to
	// marshal yaml from ? Instead of direct generation of a
	// string ?

	configurationNames := `[]`
	if len(parameters.Configurations) > 0 {
		configurationNames = fmt.Sprintf(`["%s"]`, strings.Join(parameters.Configurations, `","`))
	}

	environment := `[]`
	if len(parameters.Environment) > 0 {
		// TODO: Simplify the chain of conversions. Single `AsYAML` ?
		environment = fmt.Sprintf(`[ %s ]`, strings.Join(parameters.Environment.List().Assignments(),
			","))
	}

	routesYaml := "~"
	if len(parameters.Routes) > 0 {
		rs := []string{}
		for _, desired := range parameters.Routes {
			r := routes.FromString(desired)
			rs = append(rs, fmt.Sprintf(`{"id":"%s","domain":"%s","path":"%s"}`,
				strings.ReplaceAll(r.String(), "/", "."),
				r.Domain, r.Path))
		}
		routesYaml = fmt.Sprintf(`[%s]`, strings.Join(rs, `,`))
	}

	ingress := "~"
	name := viper.GetString("ingress-class-name")
	if name != "" {
		ingress = name
	}

	start := ""
	if parameters.Start != nil {
		start = fmt.Sprintf(`start: "%d"`, *parameters.Start)
	}

	yamlParameters := fmt.Sprintf(`
epinio:
  appName: "%[9]s"
  env: %[6]s
  imageURL: "%[3]s"
  ingress: %[10]s
  replicaCount: %[1]d
  routes: %[7]s
  configurations: %[5]s
  stageID: "%[2]s"
  tlsIssuer: "%[11]s"
  username: "%[4]s"
  %[8]s
`, parameters.Instances,
		parameters.StageID,
		parameters.ImageURL,
		parameters.Username,
		configurationNames,
		environment,
		routesYaml,
		start,
		parameters.Name,
		ingress,
		viper.GetString("tls-issuer"),
	)

	logger.Info("app helm setup", "parameters", yamlParameters)

	client, err := GetHelmClient(parameters.Cluster.RestConfig, logger, parameters.Namespace)
	if err != nil {
		return errors.Wrap(err, "create a helm client")
	}

	helmChart := appChart.HelmChart
	helmVersion := ""

	// See also part.go, fetchAppChart
	if appChart.HelmRepo != "" {
		name := names.GenerateResourceName("hr-" + base64.StdEncoding.EncodeToString([]byte(appChart.HelmRepo)))
		if err := client.AddOrUpdateChartRepo(repo.Entry{
			Name: name,
			URL:  appChart.HelmRepo,
		}); err != nil {
			return errors.Wrap(err, "creating the chart repository")
		}

		pieces := strings.SplitN(helmChart, ":", 2)
		if len(pieces) == 2 {
			helmVersion = pieces[1]
			helmChart = pieces[0]
		}

		helmChart = fmt.Sprintf("%s/%s", name, helmChart)
	}

	chartSpec := hc.ChartSpec{
		ReleaseName: names.ReleaseName(parameters.Name),
		ChartName:   helmChart,
		Version:     helmVersion,
		Recreate:    true,
		Namespace:   parameters.Namespace,
		Wait:        true,
		Atomic:      true,
		ValuesYaml:  yamlParameters,
		Timeout:     duration.ToDeployment(),
		ReuseValues: true,
	}

	if _, err := client.InstallOrUpgradeChart(context.Background(), &chartSpec, nil); err != nil {
		return err
	}

	return nil
}

func Status(ctx context.Context, logger logr.Logger, cluster *kubernetes.Cluster, namespace, releaseName string) (helmrelease.Status, error) {
	client, err := GetHelmClient(cluster.RestConfig, logger, namespace)
	if err != nil {
		return "", err
	}

	var r *helmrelease.Release
	if r, err = client.GetRelease(releaseName); err != nil {
		return "", err
	}

	if r.Info == nil {
		return "", errors.New("no status available")
	}

	return r.Info.Status, nil
}

func GetHelmClient(restConfig *rest.Config, logger logr.Logger, namespace string) (hc.Client, error) {
	options := &hc.RestConfClientOptions{
		RestConfig: restConfig,
		Options: &hc.Options{
			Namespace:        namespace,         // Match chart spec
			RepositoryCache:  "/tmp/.helmcache", // Hopefully reduces chart downloads.
			RepositoryConfig: "/tmp/.helmrepo",  // s.a.
			Linting:          true,
			Debug:            true,
			DebugLog: func(format string, v ...interface{}) {
				logger.Info("helm", "report", fmt.Sprintf(format, v...))
			},
		},
	}

	return hc.NewClientFromRestConf(options)
}
