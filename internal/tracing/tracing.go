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

// Package tracing configures OpenTelemetry tracing for the Epinio server.
package tracing

import (
	"context"
	"os"

	"github.com/epinio/epinio/internal/version"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace/noop"
)

// ServiceName is the OpenTelemetry service.name used both for the tracer
// resource and the otelgin middleware, kept as a single source of truth.
const ServiceName = "epinio-server"

// Init configures OpenTelemetry tracing for the Epinio server and returns a
// shutdown function that must be called (after the HTTP server has stopped
// serving requests) to flush and close the exporter.
//
// Tracing is opt-in: it stays a no-op unless
// OTEL_EXPORTER_OTLP_ENDPOINT (or the --otel-exporter-otlp-endpoint flag) is
// set, so an Epinio deployment without a collector behaves exactly as it
// did before this package existed.
func Init(ctx context.Context) (shutdown func(context.Context) error, err error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	noopShutdown := func(context.Context) error { return nil }

	endpoint := viper.GetString("otel-exporter-otlp-endpoint")
	if endpoint == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return noopShutdown, nil
	}

	// Materialize the resolved viper config into the process environment so
	// there is a single source of truth (the standard OTel env vars) for the
	// exporter construction below, regardless of whether the value came from
	// a CLI flag or the environment directly.
	if err := os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", endpoint); err != nil {
		return nil, errors.Wrap(err, "setting OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if protocol := viper.GetString("otel-exporter-otlp-protocol"); protocol != "" {
		if err := os.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", protocol); err != nil {
			return nil, errors.Wrap(err, "setting OTEL_EXPORTER_OTLP_PROTOCOL")
		}
	}

	exporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating OTLP span exporter")
	}

	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(ServiceName),
		semconv.ServiceVersion(version.Version),
	))
	if err != nil {
		return nil, errors.Wrap(err, "building otel resource")
	}

	tp := newTracerProvider(exporter, res)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// newTracerProvider builds a TracerProvider batching spans through the
// given exporter, tagged with the given resource. Split out from Init so
// tests can exercise it directly with an in-memory exporter.
func newTracerProvider(exporter sdktrace.SpanExporter, res *resource.Resource) *sdktrace.TracerProvider {
	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
}
