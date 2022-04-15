// Package helm contains the epinio-specific core to the helm client libraries. It exposes
// the functionality to deploy and remove helm charts/releases.
package helm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/routes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/go-logr/logr"
	hc "github.com/mittwald/go-helm-client"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	helmrelease "helm.sh/helm/v3/pkg/release"
	"k8s.io/client-go/rest"
)

const (
	StandardChart = "https://github.com/epinio/helm-charts/releases/download/epinio-application-0.1.15/epinio-application-0.1.15.tgz"
	// ChartName:   "/path/to/stable/etcd-operator",			Local directory
	// ChartName:   "/path/to/stable/etcd-operator.tar.gz",			Local archive
	// ChartName:   "http://helm.whatever.com/repo/etcd-operator.tar.gz",	Remote archive
)

type ChartParameters struct {
	models.AppRef                        // Application: name & namespace
	Cluster        *kubernetes.Cluster   // Cluster to talk to.
	Chart          string                // Chart to use for deployment
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

	chartSpec := hc.ChartSpec{
		ReleaseName: names.ReleaseName(parameters.Name),
		ChartName:   parameters.Chart,
		Namespace:   parameters.Namespace,
		Wait:        true,
		Atomic:      true,
		ValuesYaml:  yamlParameters,
		Timeout:     duration.ToDeployment(),
		ReuseValues: true,
	}

	client, err := GetHelmClient(parameters.Cluster.RestConfig, logger, parameters.Namespace)
	if err != nil {
		return err
	}

	if _, err := client.InstallOrUpgradeChart(context.Background(), &chartSpec); err != nil {
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
