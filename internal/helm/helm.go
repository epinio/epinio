// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package helm contains the epinio-specific core to the helm client libraries. It exposes
// the functionality to deploy and remove helm charts/releases.
package helm

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"

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
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/rest"
)

type ServiceParameters struct {
	models.AppRef                     // Service: name & namespace
	Context       context.Context     // Operation context
	Cluster       *kubernetes.Cluster // Cluster to talk to.
	Chart         string              // Name of helm chart to deploy
	Version       string              // Version of helm chart to deploy
	Repository    string              // Helm repository holding the chart to deploy
	Values        string              // Chart customization (YAML-formatted string)
	Wait          bool                // Wait for service to deploy
}

type ConfigParameter struct {
	Name string `yaml:"name"` // Configuration name
	Path string `yaml:"path"` // Mounting path for configuration
}

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
	Configurations []ConfigParameter     // Bound Configurations (list of names and paths)
	Routes         []string              // Desired application routes
	Domains        domain.DomainMap      // Map of domains with secrets covering them
	Start          *int64                // Nano-epoch of deployment. Optional. Used to force a restart, even when nothing else has changed.
	Settings       models.AppSettings
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

func RemoveService(logger logr.Logger, cluster *kubernetes.Cluster, app models.AppRef) error {
	client, err := GetHelmClient(cluster.RestConfig, logger, app.Namespace)
	if err != nil {
		return errors.Wrap(err, "create a helm client")
	}

	err = client.UninstallReleaseByName(names.ServiceReleaseName(app.Name))

	// Ignore errors. The release may not be present. For example due to an aborted
	// deployment. Note that a multitude of different errors was seen for essentially the same
	// thing, depending on exact timing of deletion to partial creation. Just ignoring a
	// specific one is fraught. Report, in case we were to generous and debugging is required.
	if err != nil {
		logger.Info("release deletion issue", "error", err)
	}
	return nil
}

func DeployService(logger logr.Logger, parameters ServiceParameters) error {
	logger.Info("service helm setup", "parameters", parameters)

	client, err := GetHelmClient(parameters.Cluster.RestConfig, logger, parameters.Namespace)
	if err != nil {
		return errors.Wrap(err, "create a helm client")
	}

	helmChart := parameters.Chart
	helmVersion := parameters.Version
	releaseName := names.ServiceReleaseName(parameters.Name)

	if parameters.Repository != "" {
		name := names.GenerateResourceName("hr-" + base64.StdEncoding.EncodeToString([]byte(parameters.Repository)))
		if err := client.AddOrUpdateChartRepo(repo.Entry{
			Name: name,
			URL:  parameters.Repository,
		}); err != nil {
			return errors.Wrap(err, "creating the chart repository")
		}

		helmChart = fmt.Sprintf("%s/%s", name, helmChart)
	}
	err = cleanupReleaseIfNeeded(logger, client, releaseName)
	if err != nil {
		return errors.Wrap(err, "cleaning up release")
	}

	chartSpec := hc.ChartSpec{
		ReleaseName: releaseName,
		ChartName:   helmChart,
		Version:     helmVersion,
		Namespace:   parameters.Namespace,
		Wait:        parameters.Wait,
		ValuesYaml:  string(parameters.Values),
		Timeout:     duration.ToDeployment(),
		ReuseValues: true,
	}

	_, err = client.InstallOrUpgradeChart(parameters.Context, &chartSpec, nil)

	return err
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
		Configurations []string             `yaml:"configurations"`
		ConfigPaths    []ConfigParameter    `yaml:"configpaths"`
		Env            []models.EnvVariable `yaml:"env"`
		ImageUrl       string               `yaml:"imageURL"`
		Ingress        string               `yaml:"ingress,omitempty"`
		ReplicaCount   int32                `yaml:"replicaCount"`
		Routes         []routeParam         `yaml:"routes"`
		StageID        string               `yaml:"stageID"`
		Start          string               `yaml:"start,omitempty"`
		TlsIssuer      string               `yaml:"tlsIssuer"`
		Username       string               `yaml:"username"`
	}
	type chartParam struct {
		Epinio epinioParam            `yaml:"epinio"`
		Chart  map[string]string      `yaml:"chartConfig,omitempty"`
		User   map[string]interface{} `yaml:"userConfig,omitempty"`
	}

	// Fill values.yaml structure

	// ATTENTION: The Configurations slice may contain multiple mount points for the same
	// configuration, for backward compatibility. We dedup this to have only one volume per
	// config.

	configurationNames := []string{}
	have := map[string]bool{}
	for _, c := range parameters.Configurations {
		if _, found := have[c.Name]; found {
			continue
		}
		configurationNames = append(configurationNames, c.Name)
		have[c.Name] = true
	}

	params := chartParam{
		Epinio: epinioParam{
			AppName:        parameters.Name,
			Env:            parameters.Environment.List(),
			ImageUrl:       parameters.ImageURL,
			ReplicaCount:   parameters.Instances,
			Configurations: configurationNames,
			ConfigPaths:    parameters.Configurations,
			StageID:        parameters.StageID,
			TlsIssuer:      viper.GetString("tls-issuer"),
			Username:       parameters.Username,
			// Ingress, Start, Routes: see below
		},
		// Chart, User: see below
	}

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

	// Add the settings, if any. This also performs last-minute validation.  See also
	// internal/application ValidateCV. Both use the core helper `ValidateField`
	// implemented here (Avoid import cycle).
	//
	// While nothing should trigger here we cannot be sure. Because it is currently
	// technically possible to change the app settings in the time window between a
	// client triggering validation and actually deploying the app image. This window
	// can actually be quite large, due to the time taken by staging and image
	// download.
	//
	// It doesn't even have to be malicious. Just a user B doing a normal update while
	// user A deployed, and landing in the window.

	if len(parameters.Settings) > 0 {
		params.User = make(map[string]interface{})

		for field, value := range parameters.Settings {
			spec, found := appChart.Settings[field]
			if !found {
				return fmt.Errorf("Unable to deploy. Setting '%s' unknown", field)
			}

			// Note: Here the interface{} result of the properly typed value is
			// important. It ensures that the map values are properly typed for yaml
			// serialization.

			v, err := ValidateField(field, value, spec)
			if err != nil {
				return fmt.Errorf(`Unable to deploy. %s`, err.Error())
			}
			params.User[field] = v
		}
	}

	params.Chart = appChart.Values

	// At last generate the properly quoted values.yaml string

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

	releaseName := names.ReleaseName(parameters.Name)

	err = cleanupReleaseIfNeeded(logger, client, releaseName)
	if err != nil {
		return errors.Wrap(err, "cleaning up release")
	}

	chartSpec := hc.ChartSpec{
		ReleaseName: releaseName,
		ChartName:   helmChart,
		Version:     helmVersion,
		Namespace:   parameters.Namespace,
		Wait:        true,
		Atomic:      true, // implies `Wait true`
		ValuesYaml:  string(yamlParameters),
		Timeout:     duration.ToDeployment(),
		ReuseValues: true,
	}

	_, err = client.InstallOrUpgradeChart(context.Background(), &chartSpec, nil)

	return err
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

// syncNamespaceClientMap is holding a SynchronizedClient for each namespace
var syncNamespaceClientMap sync.Map

type SynchronizedClient struct {
	namespace string
	// mutexMap is holding the mutexes for the same releases
	mutexMap   sync.Map
	helmClient hc.Client
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
	helmClient, err := hc.NewClientFromRestConf(options)
	if err != nil {
		return nil, err
	}

	return GetNamespaceSynchronizedHelmClient(namespace, helmClient)
}

func GetNamespaceSynchronizedHelmClient(namespace string, helmClient hc.Client) (*SynchronizedClient, error) {
	synchronizedHelmClient := &SynchronizedClient{
		namespace:  namespace,
		helmClient: helmClient,
	}

	// we are loading the SynchronizedClient for this namespace, if any
	loadedSynchronizedHelmClient, _ := syncNamespaceClientMap.LoadOrStore(namespace, synchronizedHelmClient)
	synchronizedHelmClient, ok := loadedSynchronizedHelmClient.(*SynchronizedClient)
	if !ok {
		return nil, errors.New("error while loading SynchronizedClient from the sync.Map")
	}

	return synchronizedHelmClient, nil
}

// cleanupReleaseIfNeeded will delete the helm release if it exists and is not
// in "deployed" state. The reason is that helm will refuse to upgrade a release
// that is in pending-install state. This would be the case, when the app container
// is failing for whatever reason. The user may try to fix the problem by pushing
// the application again and we want to allow that.
func cleanupReleaseIfNeeded(l logr.Logger, c hc.Client, name string) error {
	r, err := c.GetRelease(name)
	if err != nil {
		if err == helmdriver.ErrReleaseNotFound {
			return nil
		}
		return errors.Wrap(err, "getting the helm release")
	}

	if r.Info.Status != helmrelease.StatusDeployed {
		l.Info("Will remove existing release with status: " + string(r.Info.Status))
		err := c.UninstallRelease(&hc.ChartSpec{
			ReleaseName: name, Wait: true,
		})
		if err != nil {
			return errors.Wrapf(err, "uninstalling the release with status: %s", r.Info.Status)
		}
	}
	return nil
}

// validateField checks a single custom value against its declaration.
func ValidateField(key, value string, spec models.AppChartSetting) (interface{}, error) {
	if spec.Type == "string" {
		if len(spec.Enum) > 0 {
			for _, allowed := range spec.Enum {
				if value == allowed {
					return value, nil
				}
			}
			return nil, fmt.Errorf(`Setting "%s": Illegal string "%s"`, key, value)
		}
		return value, nil
	}
	if spec.Type == "bool" {
		flag, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf(`Setting "%s": Expected boolean, got "%s"`, key, value)
		}
		return flag, nil
	}
	if spec.Type == "integer" {
		ivalue, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf(`Setting "%s": Expected integer, got "%s"`, key, value)
		}
		return ivalue, validateRange(float64(ivalue), key, value, spec.Minimum, spec.Maximum)
	}
	if spec.Type == "number" {
		fvalue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf(`Setting "%s": Expected number, got "%s"`, key, value)
		}
		return fvalue, validateRange(fvalue, key, value, spec.Minimum, spec.Maximum)
	}

	return nil, fmt.Errorf(`Setting "%s": Bad spec: Unknown type "%s"`, key, spec.Type)
}

func validateRange(v float64, key, value, min, max string) error {
	if min != "" {
		minval, err := strconv.ParseFloat(min, 64)
		if err != nil {
			return fmt.Errorf(`Setting "%s": Bad spec: Bad minimum "%s"`, key, min)
		}
		if v < minval {
			return fmt.Errorf(`Setting "%s": Out of bounds, "%s" too small`, key, value)
		}
	}
	if max != "" {
		maxval, err := strconv.ParseFloat(max, 64)
		if err != nil {
			return fmt.Errorf(`Setting "%s": Bad spec: Bad maximum "%s"`, key, max)
		}
		if v > maxval {
			return fmt.Errorf(`Setting "%s": Out of bounds, "%s" too large`, key, value)
		}
	}
	return nil
}
