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
	"github.com/epinio/epinio/internal/domain"
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
	Domains        domain.DomainMap      // Map of domains with secrets covering them
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

	// Local type definitions for proper marshalling of the
	// `values.yaml` to hand to helm from the chart parameters.

	type routeParam struct {
		Id     string `yaml:"id"`
		Domain string `yaml:"domain"`
		Path   string `yaml:"path"`
		Secret string `yaml:"secret,omitempty"`
	}
	type epinioParam struct {
		AppName        string               `yaml:"appName"`
		Env            []models.EnvVariable `yaml:"env"`
		ImageUrl       string               `yaml:"imageURL"`
		Ingress        string               `yaml:"ingress,omitempty"`
		ReplicaCount   int32                `yaml:"replicaCount"`
		Routes         []routeParam         `yaml:"routes"`
		Configurations []string             `yaml:"configurations"`
		StageID        string               `yaml:"stageID"`
		TlsIssuer      string               `yaml:"tlsIssuer"`
		Username       string               `yaml:"username"`
		Start          string               `yaml:"start,omitempty"`
	}
	type chartParam struct {
		Epinio epinioParam `yaml:"epinio"`
	}

	// Fill values.yaml structure

	params := chartParam{
		Epinio: epinioParam{
			AppName:        parameters.Name,
			Env:            parameters.Environment.List(),
			ImageUrl:       parameters.ImageURL,
			ReplicaCount:   parameters.Instances,
			Configurations: parameters.Configurations,
			StageID:        parameters.StageID,
			TlsIssuer:      viper.GetString("tls-issuer"),
			Username:       parameters.Username,
			// Ingress, Start, Routes: see below
		},
	}

	// TODO: Is this properly nulled if the class is not set ?
	name := viper.GetString("ingress-class-name")
	if name != "" {
		params.Epinio.Ingress = name
	}
	if parameters.Start != nil {
		params.Epinio.Start = fmt.Sprintf(`%d`, *parameters.Start)
	}
	if len(parameters.Routes) > 0 {
		logger.Info("routes and domains")

		for _, desired := range parameters.Routes {
			r := routes.FromString(desired)
			rdot := strings.ReplaceAll(r.String(), "/", ".")

			rp := routeParam{
				Id:     rdot,
				Domain: r.Domain,
				Path:   r.Path,
			}

			domainSecret, err := domain.MatchDo(r.Domain, parameters.Domains)

			logger.Info("domain match", "domain", r.Domain, "secret", domainSecret, "err", err)

			// Should we treat a match error as something to stop for?
			// The error can only come from `filepath.Match()`
			if err == nil && domainSecret != "" {
				// Pass the found secret
				rp.Secret = domainSecret
			}
			params.Epinio.Routes = append(params.Epinio.Routes, rp)
		}
	}

	// And generate the properly quoted values.yaml string

	logger.Info("app helm setup", "parameters", params)

	yamlParameters, err := yaml.Marshal(params)
	if err != nil {
		return errors.Wrap(err, "marshalling the parameters")
	}

	logger.Info("app helm setup", "parameters-as-yaml", string(yamlParameters))

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
		ValuesYaml:  string(yamlParameters),
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
