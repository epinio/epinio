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
	"time"

	"github.com/pkg/errors"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/appchart"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/domain"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/routes"
	"github.com/epinio/epinio/internal/urlcache"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/go-logr/logr"
	hc "github.com/mittwald/go-helm-client"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	"helm.sh/helm/v3/pkg/registry"
	helmrelease "helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

type PostDeployFunction func(ctx context.Context) error

type ServiceParameters struct {
	models.AppRef                        // Service: name & namespace
	Cluster        *kubernetes.Cluster   // Cluster to talk to.
	CatalogService models.CatalogService // CatalogService to deploy
	Values         string                // Chart customization (YAML-formatted string)
	Wait           bool                  // Wait for service to deploy
	PostDeployHook PostDeployFunction    // Hook to call after service deployment
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
	Settings       models.ChartValueSettings
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
	return errors.Wrap(err, "deleting release")
}

func DeployService(ctx context.Context, parameters ServiceParameters) error {
	logger := requestctx.Logger(ctx)
	logger.Info("service helm setup", "parameters", parameters)

	client, err := GetHelmClient(parameters.Cluster.RestConfig, logger, parameters.Namespace)
	if err != nil {
		return errors.Wrap(err, "create a helm client")
	}

	// cleanup old release
	releaseName := names.ServiceReleaseName(parameters.Name)
	err = cleanupReleaseIfNeeded(logger, client, releaseName)
	if err != nil {
		return errors.Wrap(err, "cleaning up release")
	}

	catalogService := parameters.CatalogService

	// helmChart is the full helmChart name
	// i.e.: epinio/mychart, or oci://registry/chart for OCI charts
	// This will also login into the OCI registry or add/update the helm repository
	helmChart, err := initHelmOCIRegistryOrRepository(client, catalogService)
	if err != nil {
		return errors.Wrap(err, "initializing Helm repository or OCI registry")
	}

	chartSpec := hc.ChartSpec{
		ReleaseName: releaseName,
		ChartName:   helmChart,
		Version:     catalogService.ChartVersion,
		Namespace:   parameters.Namespace,
		Wait:        parameters.Wait,
		ValuesYaml:  string(parameters.Values),
		Timeout:     duration.ToDeployment(),
		ReuseValues: true,
	}

	if !parameters.Wait {
		// Note: We are backgrounding the action. The incoming context cannot be used, as it
		// is linked to the request. We will get a `context canceled` error. To avoid this a
		// background context is used instead.
		go func() {
			err = installOrUpgradeChartWithRetry(context.Background(), logger, client, &chartSpec)
			if err != nil {
				logger.Error(err, "installing or upgrading service ASYNC")
				return
			}

			if parameters.PostDeployHook != nil {
				// MAYBE : `wait for the release to be in a ready state` here too, see below.
				// So far, local, things were fast enough to not need it for labeling
				err := parameters.PostDeployHook(context.Background())
				if err != nil {
					logger.Error(err, "service post deployment ASYNC")
					return
				}
			}

			logger.Info("completed service deployment ASYNC")
		}()
		return nil
	}

	// Note: Steps 1: Retry only once!
	err = installOrUpgradeChartWithRetry(ctx, logger, client, &chartSpec)
	if err != nil {
		return errors.Wrap(err, "installing or upgrading service SYNC")
	}

	// wait for the release to be in a ready state
	timeout := duration.ToDeployment()
	err = wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		release, err := Release(ctx, logger, parameters.Cluster, parameters.Namespace, releaseName)
		if err != nil {
			return false, err
		}
		releaseStatus, err := Status(ctx, logger, parameters.Cluster, release)
		if releaseStatus == StatusUnknown || err != nil {
			return false, err
		}

		// check readiness
		return releaseStatus == StatusReady, nil
	})

	if err != nil {
		return errors.Wrap(err, "polling release status")
	}

	if parameters.PostDeployHook != nil {
		err := parameters.PostDeployHook(ctx)
		if err != nil {
			logger.Error(err, "service post deployment SYNC")
			return errors.Wrap(err, "post deployment action")
		}
	}

	logger.Info("completed service deployment SYNC")
	return nil
}

// initHelmOCIRegistryOrRepository will perform the initial setup for the helm Client and the specified Service,
// and return the final helm chart name. If the service is from an OCI registry it will perform a login, if needed,
// and return a 'oci://repoURL/chart' value. Else if a standard repoURL is provided it will add the repo, caching the index,
// and updating the repositories if the chart is not found.
func initHelmOCIRegistryOrRepository(client *SynchronizedClient, service models.CatalogService) (string, error) {
	// chart to install
	chartName := service.HelmChart
	chartVersion := service.ChartVersion

	// repo settings
	repoName := service.HelmRepo.Name
	repoURL := service.HelmRepo.URL

	// auth
	username := service.HelmRepo.Auth.Username
	password := service.HelmRepo.Auth.Password

	// if no friendly name was given we can hash the repoURL
	if repoName == "" {
		repoName = base64.RawURLEncoding.EncodeToString([]byte(repoURL))
	}

	// if repoURL is not set just return the 'repoName/chart'
	if repoURL == "" {
		return fmt.Sprintf("%s/%s", repoName, chartName), nil
	}

	// for an OCI registry check if we need to login and return the 'oci://registry/chart' chart name
	if registry.IsOCI(repoURL) {
		// if auth credentials are available try to login
		if username != "" && password != "" {
			registryHostname := strings.TrimPrefix(repoURL, "oci://")
			err := client.RegistryLogin(registryHostname, username, password)
			if err != nil {
				return "", errors.Wrap(err, "logging into the helm registry")
			}
		}

		// with OCI install with 'helm install foo oci://registry/chart'
		return fmt.Sprintf("%s/%s", repoURL, chartName), nil
	}

	// for all the other case we add the repo (if it doesn't exists)
	err := client.AddOrUpdateChartRepo(repo.Entry{
		Name: repoName,
		URL:  repoURL,
		// support for private repositories
		Username: username,
		Password: password,
	})
	if err != nil {
		return "", errors.Wrap(err, "creating the chart repository")
	}

	// because of this PR (https://github.com/mittwald/go-helm-client/pull/177) the previous AddOrUpdateChartRepo
	// method will NOT update the repository, so we need to check if the chart is available.
	// If not we can try to update the repos and check again
	helmChart := fmt.Sprintf("%s/%s", repoName, chartName)

	chartOpts := &action.ChartPathOptions{
		Username: username,
		Password: password,
		Version:  chartVersion,
	}

	_, _, err = client.GetChart(helmChart, chartOpts)
	// probably chart not found, try to update repositories
	if err != nil {
		if err := client.UpdateChartRepos(); err != nil {
			return "", errors.Wrap(err, "updating the chart repositories")
		}

		if _, _, err = client.GetChart(helmChart, chartOpts); err != nil {
			return "", errors.Wrapf(err, "looking for the '%s' chart [version: %s] [repoURL: %s]", helmChart, chartVersion, repoURL)
		}
	}

	return helmChart, nil
}

// Local type definitions for proper marshalling of the
// `values.yaml` to hand to helm from the chart parameters.

type RouteParam struct {
	Id     string `yaml:"id"`
	Domain string `yaml:"domain"`
	Path   string `yaml:"path"`
	Secret string `yaml:"secret,omitempty"`
}
type EpinioParam struct {
	AppName        string               `yaml:"appName"`
	Configurations []string             `yaml:"configurations"`
	ConfigPaths    []ConfigParameter    `yaml:"configpaths"`
	Env            []models.EnvVariable `yaml:"env"`
	ImageUrl       string               `yaml:"imageURL"`
	Ingress        string               `yaml:"ingress,omitempty"`
	ReplicaCount   int32                `yaml:"replicaCount"`
	Routes         []RouteParam         `yaml:"routes"`
	StageID        string               `yaml:"stageID"`
	Start          string               `yaml:"start,omitempty"`
	TlsIssuer      string               `yaml:"tlsIssuer"`
	Username       string               `yaml:"username"`
}
type ChartParam struct {
	Epinio EpinioParam            `yaml:"epinio"`
	Chart  map[string]string      `yaml:"chartConfig,omitempty"`
	User   map[string]interface{} `yaml:"userConfig,omitempty"`
}

func Deploy(logger logr.Logger, parameters ChartParameters) error {
	logger.Info("deploy app", "parameters", parameters)

	// Find the app chart to use for the deployment.
	appChart, err := appchart.Lookup(parameters.Context, parameters.Cluster, parameters.Chart)
	if err != nil {
		return errors.Wrap(err, "looking up application chart")
	}
	if appChart == nil {
		return fmt.Errorf("Unable to deploy, chart %s not found", parameters.Chart)
	}

	logger.Info("deploy app", "appchart", appChart)

	// Fill values.yaml structure

	params, err := getValuesYAML(logger, appChart, parameters)
	if err != nil {
		return err
	}

	client, err := GetHelmClient(parameters.Cluster.RestConfig, logger, parameters.Namespace)
	if err != nil {
		return errors.Wrap(err, "create a helm client")
	}

	helmChart, helmVersion, err := getChartReference(parameters.Context, logger, client, appChart)
	if err != nil {
		return errors.Wrap(err, "retrieving chart reference")
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
		ValuesYaml:  params,
		Timeout:     duration.ToDeployment(),
		ReuseValues: true,
	}

	_, err = client.InstallOrUpgradeChart(context.Background(), &chartSpec, nil)

	return err
}

// Status is the status of a release
type ReleaseStatus string

const (
	// StatusUnknown indicates that a release is in an uncertain state.
	StatusUnknown ReleaseStatus = "unknown"
	// StatusReady indicates that all the release's resources are in a ready state.
	StatusReady ReleaseStatus = "ready"
	// StatusNotReady indicates that not all the release's resources are in a ready state.
	StatusNotReady ReleaseStatus = "not-ready"
)

// Release returns the named Helm release, including the associated resources. This is the internal
// equivalent of the `helm status` command.
func Release(ctx context.Context, logger logr.Logger, cluster *kubernetes.Cluster,
	namespace, releaseName string) (*helmrelease.Release, error) {

	helmClient, err := GetHelmClient(cluster.RestConfig, logger, namespace)
	if err != nil {
		return nil, err
	}

	release, err := helmClient.Status(releaseName)
	if err != nil {
		return nil, errors.Wrapf(err, "getting release status %s - %s", namespace, releaseName)
	}

	return release, err
}

// Status will check for the readiness of the release returning an internal status instead of
// the Helm release status (https://github.com/helm/helm/blob/main/pkg/release/status.go).
// Helm is not checking for the actual status of the release and even if the resources are still
// in deployment they will be marked as "deployed"
func Status(ctx context.Context, logger logr.Logger, cluster *kubernetes.Cluster,
	release *helmrelease.Release) (ReleaseStatus, error) {
	resourceList := getResourceListFromRelease(release)
	logger.V(1).Info(fmt.Sprintf(
		"found '%d' resources for release '%s' in namespace '%s'\n",
		len(resourceList), release.Name, release.Namespace),
	)

	checker := kube.NewReadyChecker(cluster.Kubectl, logger.Info, kube.PausedAsReady(true))
	for _, v := range resourceList {
		// IsReady checks if v is ready. It supports checking readiness for pods,
		// deployments, persistent volume claims, services, daemon sets, custom
		// resource definitions, stateful sets, replication controllers, and replica
		// sets. All other resource kinds are always considered ready.
		ready, err := checker.IsReady(ctx, v)

		logger.V(1).Info(fmt.Sprintf("resource '%s' ready: '%t'\n", v.Name, ready))

		if err != nil {
			return StatusUnknown, errors.Wrapf(err,
				"checking readiness of resource '%s' of release '%s'",
				v.Name, release.Name)
		}
		if !ready {
			return StatusNotReady, nil
		}
	}

	return StatusReady, nil
}

// getResourcesFromRelease will look for Unstructured resources in the release and will return a list out of it
func getResourceListFromRelease(release *helmrelease.Release) kube.ResourceList {
	resourceList := make(kube.ResourceList, 0)

	for _, objectList := range release.Info.Resources {
		for _, obj := range objectList {
			if v, ok := obj.(*unstructured.Unstructured); ok {
				resourceList = append(resourceList, &resource.Info{
					Object:    obj,
					Name:      v.GetName(),
					Namespace: v.GetNamespace(),
				})
			}

		}
	}

	return resourceList
}

// syncNamespaceClientMap is holding a SynchronizedClient for each namespace
var syncNamespaceClientMap sync.Map

type SynchronizedClient struct {
	namespace string
	// mutexMap is holding the mutexes for the same releases
	mutexMap   sync.Map
	helmClient hc.Client
}

func GetHelmClient(restConfig *rest.Config, logger logr.Logger, namespace string) (*SynchronizedClient, error) {
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

	if r.Info.Status == helmrelease.StatusDeployed {
		return nil
	}

	l.Info("Will remove existing release with status: " + string(r.Info.Status))
	err = c.UninstallRelease(&hc.ChartSpec{
		ReleaseName: name,
		Wait:        true,
	})

	if err != nil {
		l.Error(err, fmt.Sprintf("uninstalling the release with status: %s", r.Info.Status))

		// Sometimes we get an error but the release was uninstalled anyway.
		// Check again if the release exists.
		r, errGet := c.GetRelease(name)
		if errGet != nil {
			if errGet == helmdriver.ErrReleaseNotFound {
				return nil
			}
			return errors.Wrap(errGet, "getting the helm release after uninstall")
		}

		// The release still exists, return the original error
		return errors.Wrapf(err, "uninstalling the release with status: %s", r.Info.Status)
	}
	return nil
}

// validateField checks a single custom value against its declaration.
func ValidateField(key, value string, spec models.ChartSetting) (interface{}, error) {
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

func installOrUpgradeChartWithRetry(ctx context.Context, logger logr.Logger, client hc.Client,
	chartSpec *hc.ChartSpec) error {

	// This, the retry, should fix issue https://github.com/epinio/epinio/issues/2385

	backoff := wait.Backoff{
		Steps:    2, // Retry only once (2 tries)!
		Duration: 10 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}

	alwaysRetry := func(error) bool { return true }

	return retry.OnError(backoff, alwaysRetry, func() error {
		_, err := client.InstallOrUpgradeChart(ctx, chartSpec, nil)
		if err != nil {
			logger.Error(err, "installing or upgrading service, retry")
		}
		return err
	})
}

func getChartReference(ctx context.Context, logger logr.Logger, client hc.Client, appChart *models.AppChartFull) (string, string, error) {
	// chart, version, error
	// See also part.go, fetchAppChart

	logger.Info("deploy app", "raw-appchart", appChart.HelmChart, "helm-repo", appChart.HelmRepo)

	if appChart.HelmRepo == "" {
		// The helm chart is either a local file, or a direct url to the chart location.

		logger.Info("deploy app", "appchart-url", appChart.HelmChart)

		helmChart, err := urlcache.Get(ctx, logger, appChart.HelmChart)
		if err != nil {
			return "", "", err
		}

		logger.Info("deploy app", "appchart-file", helmChart)
		return helmChart, "", nil
	}

	// The helm chart ref is a name in a repository.
	// This name may have a version appended to it, separated from the actual chart by `:`.
	// Ensure that the repository is locally known.

	repositoryName := names.GenerateResourceName("hr-" + base64.StdEncoding.EncodeToString([]byte(appChart.HelmRepo)))
	if err := client.AddOrUpdateChartRepo(repo.Entry{
		Name: repositoryName,
		URL:  appChart.HelmRepo,
	}); err != nil {
		return "", "", errors.Wrap(err, "creating the chart repository")
	}

	// Decode the chart ref. IOW extract the possible version tag.

	helmChart := appChart.HelmChart
	helmVersion := ""
	pieces := strings.SplitN(helmChart, ":", 2)
	if len(pieces) == 2 {
		helmVersion = pieces[1]
		helmChart = pieces[0]
	}

	// Combine chart and repository to a proper in-repo reference.

	helmChart = fmt.Sprintf("%s/%s", repositoryName, helmChart)

	logger.Info("deploy app", "appchart", helmChart, "version", helmVersion)

	return helmChart, helmVersion, nil
}

func getValuesYAML(logger logr.Logger, appChart *models.AppChartFull, parameters ChartParameters) (string, error) {
	logger.Info("deploy app, get values.yaml")

	// ATTENTION: The configurations slice may contain multiple mount points for the same
	// configuration, for backward compatibility. We dedup this so that there is only one volume
	// per configuration.

	configurationNames := []string{}
	have := map[string]bool{}
	for _, c := range parameters.Configurations {
		if _, found := have[c.Name]; found {
			continue
		}
		configurationNames = append(configurationNames, c.Name)
		have[c.Name] = true
	}

	params := ChartParam{
		Epinio: EpinioParam{
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
		logger.Info("deploy app", "ingress-class", name)
	}
	if parameters.Start != nil {
		params.Epinio.Start = fmt.Sprintf(`%d`, *parameters.Start)
		logger.Info("deploy app", "start", params.Epinio.Start)
	}
	if len(parameters.Routes) > 0 {
		logger.Info("deploy app, routes and domains")

		for _, desired := range parameters.Routes {
			r := routes.FromString(desired)
			rdot := strings.ReplaceAll(r.String(), "/", ".")

			rp := RouteParam{
				Id:     rdot,
				Domain: r.Domain,
				Path:   r.Path,
			}

			domainSecret, err := domain.MatchDo(r.Domain, parameters.Domains)

			logger.Info("deploy app, domain match", "domain", r.Domain,
				"secret", domainSecret, "err", err)

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
		logger.Info("deploy app, settings")

		params.User = make(map[string]interface{})

		for field, value := range parameters.Settings {
			spec, found := appChart.Settings[field]
			if !found {
				return "", fmt.Errorf("Unable to deploy. Setting '%s' unknown", field)
			}

			// Note: Here the interface{} result of the properly typed value is
			// important. It ensures that the map values are properly typed for yaml
			// serialization.

			v, err := ValidateField(field, value, spec)
			if err != nil {
				return "", fmt.Errorf(`Unable to deploy. %s`, err.Error())
			}
			params.User[field] = v

			logger.Info("deploy app, settings", "field", field, "value", v)
		}
	}

	params.Chart = appChart.Values

	// At last generate the properly quoted values.yaml string

	logger.Info("deploy app", "helm values.raw", params)

	yamlParameters, err := yaml.Marshal(params)
	if err != nil {
		return "", errors.Wrap(err, "marshalling the parameters")
	}

	yamlString := string(yamlParameters)

	logger.Info("deploy app", "helm values.yaml", yamlString)
	logger.Info("deploy app, return values.yaml")
	return yamlString, nil
}
