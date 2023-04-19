package helm

import (
	"context"
	"sync"

	hc "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	helmrelease "helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
)

var _ hc.Client = (*SynchronizedClient)(nil)

// InstallOrUpgradeChart implements helmclient.Client
func (c *SynchronizedClient) InstallOrUpgradeChart(ctx context.Context, spec *hc.ChartSpec, opts *hc.GenericHelmOptions) (*release.Release, error) {
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
