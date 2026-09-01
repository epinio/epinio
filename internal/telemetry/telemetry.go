// Copyright © 2026 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package telemetry gathers a daily fleet inventory snapshot (Epinio
// version, Kubernetes version, app/namespace/service counts) and pushes it
// to Grafana Cloud over OTLP/HTTP. It is triggered once a day by the
// epinio-telemetry CronJob calling POST /api/v1/telemetry/publish.
package telemetry

import (
	"context"
	"encoding/base64"
	"net/url"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/instance"
	"github.com/epinio/epinio/internal/namespaces"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/internal/version"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Snapshot is a single fleet inventory reading. JSON tags let the API handler
// echo it back in the publish response, so it can be inspected (e.g. from
// Postman) without needing to open Grafana Cloud.
type Snapshot struct {
	CollectedAt   time.Time `json:"timestamp"`
	EpinioVersion string    `json:"epinio_version"`
	ChartVersion  string    `json:"chart_version"`
	KubeVersion   string    `json:"kube_version"`
	Platform      string    `json:"platform"`
	InstanceID    string    `json:"instance_id"`
	InstallMethod string    `json:"install_method"`
	Applications  int       `json:"applications"`
	Namespaces    int       `json:"namespaces"`
	Services      int       `json:"services"`
}

// Config carries the Grafana Cloud OTLP destination and auth. All fields are
// required for Push to succeed; ClusterLabel/EnvironmentLabel are optional.
type Config struct {
	OTLPEndpoint      string
	GrafanaInstanceID string
	GrafanaToken      string
	ClusterLabel      string
	EnvironmentLabel  string
}

// Collect gathers the current fleet inventory snapshot. It reuses the same
// helpers already used by GET /api/v1/info and the fleet report endpoint, so
// it makes no Kubernetes calls beyond what epinio-server already performs
// elsewhere.
func Collect(ctx context.Context, cluster *kubernetes.Cluster) (Snapshot, error) {
	kubeVersion, err := cluster.GetVersion()
	if err != nil {
		return Snapshot{}, errors.Wrap(err, "getting kube version")
	}

	instanceInfo, err := instance.GetCachedOrCreate(ctx, cluster)
	if err != nil {
		return Snapshot{}, errors.Wrap(err, "getting instance info")
	}

	appRefs, err := application.ListAppRefs(ctx, cluster, "")
	if err != nil {
		return Snapshot{}, errors.Wrap(err, "listing applications")
	}

	nsList, err := namespaces.List(ctx, cluster)
	if err != nil {
		return Snapshot{}, errors.Wrap(err, "listing namespaces")
	}

	svcClient, err := services.NewKubernetesServiceClient(cluster)
	if err != nil {
		return Snapshot{}, errors.Wrap(err, "creating service client")
	}

	svcList, err := svcClient.ListAll(ctx)
	if err != nil {
		return Snapshot{}, errors.Wrap(err, "listing services")
	}

	return Snapshot{
		CollectedAt:   time.Now().UTC(),
		EpinioVersion: version.Version,
		ChartVersion:  version.ChartVersion,
		KubeVersion:   kubeVersion,
		Platform:      cluster.GetPlatform().String(),
		InstanceID:    instanceInfo.ID,
		InstallMethod: instanceInfo.InstallMethod,
		Applications:  len(appRefs),
		Namespaces:    len(nsList),
		Services:      len(svcList),
	}, nil
}

// Push performs a single OTLP/HTTP metrics export of the snapshot to Grafana
// Cloud (Mimir), authenticating with HTTP Basic auth (Grafana Cloud instance
// ID as username, access-policy token as password). It builds a short-lived
// MeterProvider, records one data point per gauge, force-flushes it (which
// triggers the actual HTTP export), and shuts the provider down again -
// there is no long-running metrics pipeline here, just a single push.
func Push(ctx context.Context, snapshot Snapshot, cfg Config) error {
	if cfg.OTLPEndpoint == "" {
		return errors.New("telemetry: grafana cloud otlp endpoint is not configured")
	}
	if cfg.GrafanaInstanceID == "" || cfg.GrafanaToken == "" {
		return errors.New("telemetry: grafana cloud credentials are not configured")
	}

	endpointURL, err := url.Parse(cfg.OTLPEndpoint)
	if err != nil {
		return errors.Wrap(err, "parsing otlp endpoint")
	}

	path := endpointURL.Path
	if path == "" || path == "/" {
		path = "/v1/metrics"
	}

	authHeader := "Basic " + base64.StdEncoding.EncodeToString(
		[]byte(cfg.GrafanaInstanceID+":"+cfg.GrafanaToken),
	)

	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(endpointURL.Host),
		otlpmetrichttp.WithURLPath(path),
		otlpmetrichttp.WithHeaders(map[string]string{"Authorization": authHeader}),
		otlpmetrichttp.WithTimeout(15 * time.Second),
	}
	if endpointURL.Scheme == "http" {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return errors.Wrap(err, "creating otlp exporter")
	}

	// A long interval is fine: we never let the periodic timer fire on its
	// own, ForceFlush below drives the one and only export.
	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(time.Hour))
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = provider.Shutdown(shutdownCtx)
	}()

	meter := provider.Meter("github.com/epinio/epinio/internal/telemetry")

	buildInfoAttrs := attribute.NewSet(
		attribute.String("epinio_version", strings.TrimPrefix(snapshot.EpinioVersion, "v")),
		attribute.String("chart_version", strings.TrimPrefix(snapshot.ChartVersion, "v")),
		attribute.String("kubernetes_version", snapshot.KubeVersion),
		attribute.String("platform", snapshot.Platform),
		attribute.String("instance_id", snapshot.InstanceID),
		attribute.String("install_method", snapshot.InstallMethod),
		attribute.String("cluster", cfg.ClusterLabel),
		attribute.String("environment", cfg.EnvironmentLabel),
	)

	inventoryAttrs := attribute.NewSet(
		attribute.String("instance_id", snapshot.InstanceID),
		attribute.String("cluster", cfg.ClusterLabel),
		attribute.String("environment", cfg.EnvironmentLabel),
	)

	registerGauge := func(name, description string, value float64, attrs attribute.Set) error {
		_, err := meter.Float64ObservableGauge(name,
			metric.WithDescription(description),
			metric.WithFloat64Callback(func(_ context.Context, o metric.Float64Observer) error {
				o.Observe(value, metric.WithAttributeSet(attrs))
				return nil
			}),
		)
		return errors.Wrapf(err, "registering %s gauge", name)
	}

	if err := registerGauge("epinio_build_info", "Epinio installation identity, always 1", 1, buildInfoAttrs); err != nil {
		return err
	}
	if err := registerGauge("epinio_inventory_applications", "Number of Epinio applications", float64(snapshot.Applications), inventoryAttrs); err != nil {
		return err
	}
	if err := registerGauge("epinio_inventory_namespaces", "Number of Epinio namespaces", float64(snapshot.Namespaces), inventoryAttrs); err != nil {
		return err
	}
	if err := registerGauge("epinio_inventory_services", "Number of Epinio services", float64(snapshot.Services), inventoryAttrs); err != nil {
		return err
	}
	if err := registerGauge("epinio_telemetry_last_success_timestamp", "Unix timestamp of the last successful telemetry push", float64(snapshot.CollectedAt.Unix()), inventoryAttrs); err != nil {
		return err
	}

	if err := provider.ForceFlush(ctx); err != nil {
		return errors.Wrap(err, "flushing telemetry to grafana cloud")
	}

	return nil
}
