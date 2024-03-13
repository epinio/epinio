package helm

import (
	"context"
	"fmt"
	"sync"

	hc "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	helmrelease "helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
)

var _ hc.Client = (*SynchronizedClient)(nil)

// InstallOrUpgradeChart implements helmclient.Client
func (c *SynchronizedClient) InstallOrUpgradeChart(ctx context.Context, spec *hc.ChartSpec, opts *hc.GenericHelmOptions) (*helmrelease.Release, error) {
	anyMutex, _ := c.mutexMap.LoadOrStore(spec.ReleaseName, &sync.Mutex{})
	if m, ok := anyMutex.(*sync.Mutex); ok {
		m.Lock()
		defer m.Unlock()
	}

	return c.helmClient.InstallOrUpgradeChart(ctx, spec, opts)
}

// RollbackRelease implements helmclient.Client
func (c *SynchronizedClient) RollbackRelease(spec *hc.ChartSpec) error {
	anyMutex, _ := c.mutexMap.LoadOrStore(spec.ReleaseName, &sync.Mutex{})
	if m, ok := anyMutex.(*sync.Mutex); ok {
		m.Lock()
		defer m.Unlock()
	}

	return c.helmClient.RollbackRelease(spec)
}

// AddOrUpdateChartRepo implements helmclient.Client
func (c *SynchronizedClient) AddOrUpdateChartRepo(entry repo.Entry) error {
	return c.helmClient.AddOrUpdateChartRepo(entry)
}

// GetChart implements helmclient.Client
func (c *SynchronizedClient) GetChart(chartName string, chartPathOptions *action.ChartPathOptions) (*chart.Chart, string, error) {
	return c.helmClient.GetChart(chartName, chartPathOptions)
}

// GetRelease implements helmclient.Client
func (c *SynchronizedClient) GetRelease(name string) (*helmrelease.Release, error) {
	return c.helmClient.GetRelease(name)
}

// GetReleaseValues implements helmclient.Client
func (c *SynchronizedClient) GetReleaseValues(name string, allValues bool) (map[string]interface{}, error) {
	return c.helmClient.GetReleaseValues(name, allValues)
}

// InstallChart implements helmclient.Client
func (c *SynchronizedClient) InstallChart(ctx context.Context, spec *hc.ChartSpec, opts *hc.GenericHelmOptions) (*helmrelease.Release, error) {
	anyMutex, _ := c.mutexMap.LoadOrStore(spec.ReleaseName, &sync.Mutex{})
	if m, ok := anyMutex.(*sync.Mutex); ok {
		m.Lock()
		defer m.Unlock()
	}

	return c.helmClient.InstallChart(ctx, spec, opts)
}

// LintChart implements helmclient.Client
func (c *SynchronizedClient) LintChart(spec *hc.ChartSpec) error {
	return c.helmClient.LintChart(spec)
}

// ListDeployedReleases implements helmclient.Client
func (c *SynchronizedClient) ListDeployedReleases() ([]*helmrelease.Release, error) {
	return c.helmClient.ListDeployedReleases()
}

// ListReleaseHistory implements helmclient.Client
func (c *SynchronizedClient) ListReleaseHistory(name string, max int) ([]*helmrelease.Release, error) {
	return c.helmClient.ListReleaseHistory(name, max)
}

// ListReleasesByStateMask implements helmclient.Client
func (c *SynchronizedClient) ListReleasesByStateMask(actions action.ListStates) ([]*helmrelease.Release, error) {
	return c.helmClient.ListReleasesByStateMask(actions)
}

// SetDebugLog implements helmclient.Client
func (c *SynchronizedClient) SetDebugLog(debugLog action.DebugLog) {
	c.helmClient.SetDebugLog(debugLog)
}

// TemplateChart implements helmclient.Client
func (c *SynchronizedClient) TemplateChart(spec *hc.ChartSpec, options *hc.HelmTemplateOptions) ([]byte, error) {
	return c.helmClient.TemplateChart(spec, options)
}

// UninstallRelease implements helmclient.Client
func (c *SynchronizedClient) UninstallRelease(spec *hc.ChartSpec) error {
	anyMutex, _ := c.mutexMap.LoadOrStore(spec.ReleaseName, &sync.Mutex{})
	if m, ok := anyMutex.(*sync.Mutex); ok {
		m.Lock()
		defer m.Unlock()
	}

	return c.helmClient.UninstallRelease(spec)
}

// UninstallReleaseByName implements helmclient.Client
func (c *SynchronizedClient) UninstallReleaseByName(name string) error {
	anyMutex, _ := c.mutexMap.LoadOrStore(name, &sync.Mutex{})
	if m, ok := anyMutex.(*sync.Mutex); ok {
		m.Lock()
		defer m.Unlock()
	}

	return c.helmClient.UninstallReleaseByName(name)
}

// UpdateChartRepos implements helmclient.Client
func (c *SynchronizedClient) UpdateChartRepos() error {
	return c.helmClient.UpdateChartRepos()
}

// UpgradeChart implements helmclient.Client
func (c *SynchronizedClient) UpgradeChart(ctx context.Context, spec *hc.ChartSpec, opts *hc.GenericHelmOptions) (*helmrelease.Release, error) {
	anyMutex, _ := c.mutexMap.LoadOrStore(spec.ReleaseName, &sync.Mutex{})
	if m, ok := anyMutex.(*sync.Mutex); ok {
		m.Lock()
		defer m.Unlock()
	}

	return c.helmClient.UpgradeChart(ctx, spec, opts)
}

// Status implements the 'helm status' command, with the ShowResources flag enabled
func (c *SynchronizedClient) Status(name string) (*helmrelease.Release, error) {
	concreteHelmClient, ok := c.helmClient.(*hc.HelmClient)
	if !ok {
		return nil, fmt.Errorf("helm client is not of the right type. Expected *hc.HelmClient but got %T", c.helmClient)
	}

	statusAction := action.NewStatus(concreteHelmClient.ActionConfig)
	statusAction.ShowResources = true
	return statusAction.Run(name)
}

func (c *SynchronizedClient) RegistryLogin(hostname, username, password string, opts ...action.RegistryLoginOpt) error {
	concreteHelmClient, ok := c.helmClient.(*hc.HelmClient)
	if !ok {
		return fmt.Errorf("helm client is not of the right type. Expected *hc.HelmClient but got %T", c.helmClient)
	}

	registryLoginAction := action.NewRegistryLogin(concreteHelmClient.ActionConfig)
	return registryLoginAction.Run(nil, hostname, username, password, opts...)
}

func (c *SynchronizedClient) Push(chartref, remote string, opts ...action.PushOpt) (string, error) {
	concreteHelmClient, ok := c.helmClient.(*hc.HelmClient)
	if !ok {
		return "", fmt.Errorf("helm client is not of the right type. Expected *hc.HelmClient but got %T", c.helmClient)
	}

	ac := *concreteHelmClient.ActionConfig
	ac.RegistryClient = nil

	opts = append(opts, action.WithPushConfig(&ac))

	registryPushAction := action.NewPushWithOpts(opts...)
	return registryPushAction.Run(chartref, remote)
}

func (c *SynchronizedClient) GetProviders() getter.Providers {
	return c.helmClient.GetProviders()
}

func (c *SynchronizedClient) GetSettings() *cli.EnvSettings {
	return c.helmClient.GetSettings()
}

func (c *SynchronizedClient) RunChartTests(releaseName string) (bool, error) {
	return c.helmClient.RunChartTests(releaseName)
}
