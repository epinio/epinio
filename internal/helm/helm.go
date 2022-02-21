// Package helm contains the epinio-specific core to the helm client libraries. It exposes
// the functionality to deploy and remove helm charts/releases.
package helm

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/routes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/go-logr/logr"
	hc "github.com/mittwald/go-helm-client"
	"github.com/spf13/viper"
)

const (
	StandardChart = "https://github.com/epinio/helm-charts/releases/download/epinio-application-0.1.10/epinio-application-0.1.10.tgz"
	// ChartName:   "/path/to/stable/etcd-operator",			Local directory
	// ChartName:   "/path/to/stable/etcd-operator.tar.gz",			Local archive
	// ChartName:   "http://helm.whatever.com/repo/etcd-operator.tar.gz",	Remote archive
)

type ChartParameters struct {
	models.AppRef                       // Application: name & namespace
	Cluster       *kubernetes.Cluster   // Cluster to talk to.
	Chart         string                // Chart to use for deployment
	ImageURL      string                // Application Image
	Username      string                // User causing the (re)deployment
	Instances     int32                 // Number Of Desired Replicas
	StageID       string                // Stage ID for ImageURL
	Owner         metav1.OwnerReference // App CRD Owner Information
	Environment   models.EnvVariableMap // App Environment
	Services      []string              // Bound Services (list of names)
	Routes        []string              // Desired application routes
	Start         *int64                // Nano-epoch of deployment. Optional. Used to force a restart, even when nothing else has changed.
}

func Remove(cluster *kubernetes.Cluster, logger logr.Logger, app models.AppRef) error {
	client, err := getHelmClient(cluster, logger, app.Namespace)
	if err != nil {
		return err
	}

	return client.UninstallReleaseByName(names.ReleaseName(app.Name))
}

func Deploy(logger logr.Logger, parameters ChartParameters) error {

	// YAML string - TODO ? Use unstructured as intermediary to
	// marshal yaml from ? Instead of direct generation of a
	// string ?

	serviceNames := `[]`
	if len(parameters.Services) > 0 {
		serviceNames = fmt.Sprintf(`["%s"]`, strings.Join(parameters.Services, `","`))
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
  appName: "%[10]s"
  appUID: "%[2]s"
  env: %[7]s
  imageURL: "%[4]s"
  ingress: %[11]s
  replicaCount: %[1]d
  routes: %[8]s
  services: %[6]s
  stageID: "%[3]s"
  tlsIssuer: "%[12]s"
  username: "%[5]s"
  %[9]s
`, parameters.Instances,
		parameters.Owner.UID,
		parameters.StageID,
		parameters.ImageURL,
		parameters.Username,
		serviceNames,
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

	client, err := getHelmClient(parameters.Cluster, logger, parameters.Namespace)
	if err != nil {
		return err
	}

	if _, err := client.InstallOrUpgradeChart(context.Background(), &chartSpec); err != nil {
		return err
	}

	return nil
}

func getHelmClient(cluster *kubernetes.Cluster, logger logr.Logger, namespace string) (hc.Client, error) {
	options := &hc.RestConfClientOptions{
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
		RestConfig: cluster.RestConfig,
	}

	client, err := hc.NewClientFromRestConf(options)
	if err != nil {
		return nil, err
	}

	return client, nil
}
