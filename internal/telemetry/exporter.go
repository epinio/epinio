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

package telemetry

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Push exports the fleet inventory snapshot to Grafana Cloud via OTLP/HTTP.
func Push(ctx context.Context, cfg Config, inventory models.FleetInventory) error {
	if !cfg.Ready() {
		return errors.New("telemetry export is not configured")
	}

	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(cfg.OTLPEndpoint),
		otlpmetrichttp.WithHeaders(map[string]string{
			"Authorization": basicAuthHeader(cfg.OTLPUsername, cfg.OTLPPassword),
		}),
	)
	if err != nil {
		return errors.Wrap(err, "creating OTLP metric exporter")
	}

	reader := sdkmetric.NewManualReader()
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("epinio"),
			semconv.ServiceVersion(inventory.EpinioVersion),
			attribute.String("epinio.chart_version", inventory.EpinioChartVersion),
			attribute.String("epinio.instance_id", inventory.InstanceID),
			attribute.String("epinio.install_method", inventory.InstallMethod),
			attribute.String("cluster", inventory.Cluster),
			attribute.String("environment", inventory.Environment),
			attribute.String("kubernetes.version", inventory.KubernetesVersion),
			attribute.String("platform", inventory.Platform),
		),
	)
	if err != nil {
		return errors.Wrap(err, "creating otel resource")
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)
	defer func() {
		_ = provider.Shutdown(context.Background())
	}()

	meter := provider.Meter("github.com/epinio/epinio/internal/telemetry")

	if err := recordInventoryMetrics(ctx, meter, inventory); err != nil {
		return err
	}

	var metrics metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &metrics); err != nil {
		return errors.Wrap(err, "collecting metrics")
	}

	if err := exporter.Export(ctx, &metrics); err != nil {
		return errors.Wrap(err, "exporting metrics to OTLP endpoint")
	}

	return nil
}

func recordInventoryMetrics(ctx context.Context, meter metric.Meter, inventory models.FleetInventory) error {
	commonAttrs := metric.WithAttributes(
		attribute.String("epinio_instance_id", inventory.InstanceID),
		attribute.String("epinio_install_method", inventory.InstallMethod),
		attribute.String("epinio_version", inventory.EpinioVersion),
		attribute.String("epinio_chart_version", inventory.EpinioChartVersion),
		attribute.String("kubernetes_version", inventory.KubernetesVersion),
		attribute.String("platform", inventory.Platform),
		attribute.String("cluster", inventory.Cluster),
		attribute.String("environment", inventory.Environment),
	)

	appGauge, err := meter.Float64Gauge(
		"epinio_inventory_applications",
		metric.WithDescription("Number of Epinio applications in the cluster"),
	)
	if err != nil {
		return errors.Wrap(err, "creating applications gauge")
	}
	appGauge.Record(ctx, float64(inventory.ApplicationCount), commonAttrs)

	nsGauge, err := meter.Float64Gauge(
		"epinio_inventory_namespaces",
		metric.WithDescription("Number of Epinio-controlled namespaces in the cluster"),
	)
	if err != nil {
		return errors.Wrap(err, "creating namespaces gauge")
	}
	nsGauge.Record(ctx, float64(inventory.NamespaceCount), commonAttrs)

	svcGauge, err := meter.Float64Gauge(
		"epinio_inventory_services",
		metric.WithDescription("Number of Epinio service instances in the cluster"),
	)
	if err != nil {
		return errors.Wrap(err, "creating services gauge")
	}
	svcGauge.Record(ctx, float64(inventory.ServiceCount), commonAttrs)

	buildInfo, err := meter.Float64Gauge(
		"epinio_build_info",
		metric.WithDescription("Epinio server build information marker (value is always 1)"),
	)
	if err != nil {
		return errors.Wrap(err, "creating build info gauge")
	}
	buildInfo.Record(ctx, 1, commonAttrs)

	lastSuccess, err := meter.Float64Gauge(
		"epinio_telemetry_last_success_timestamp",
		metric.WithDescription("Unix timestamp of the last successful fleet telemetry push"),
	)
	if err != nil {
		return errors.Wrap(err, "creating last success gauge")
	}
	lastSuccess.Record(ctx, float64(time.Now().UTC().Unix()), commonAttrs)

	return nil
}

func basicAuthHeader(username, password string) string {
	token := fmt.Sprintf("%s:%s", username, password)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(token))
}
