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

// Package tracing configures OpenTelemetry tracing and log correlation for the Epinio server.
package tracing

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/version"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace/noop"
)

// ServiceName is the OpenTelemetry service.name used for the tracer
// resource, Gin/HTTP instrumentation, and the otelzap bridge.
const ServiceName = "epinio-server"

// Init configures OpenTelemetry tracing (and, when enabled, log export via
// otelzap) for the Epinio server. The returned shutdown function must be
// called after the HTTP server has stopped serving requests so exporters
// can flush.
//
// Tracing is opt-in: it stays a no-op unless OTEL_EXPORTER_OTLP_ENDPOINT,
// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT, or the --otel-exporter-otlp-endpoint
// flag is set, so an Epinio deployment without a collector behaves exactly
// as it did before this package existed.
func Init(ctx context.Context) (shutdown func(context.Context) error, err error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Otherwise otel's default handler writes straight to stderr, bypassing
	// the structured zap logging the rest of the server uses.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		if helpers.Logger != nil {
			helpers.Logger.Errorw("otel error", "error", err)
		}
	}))

	noopShutdown := func(context.Context) error { return nil }

	endpoint := viper.GetString("otel-exporter-otlp-endpoint")
	if endpoint == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		otel.SetTracerProvider(noop.NewTracerProvider())
		return noopShutdown, nil
	}

	// Materialize the resolved viper config into the process environment so
	// there is a single source of truth (the standard OTel env vars) for the
	// exporter construction below, regardless of whether the value came from
	// a CLI flag or the environment directly.
	if endpoint != "" {
		if err := os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", endpoint); err != nil {
			return nil, errors.Wrap(err, "setting OTEL_EXPORTER_OTLP_ENDPOINT")
		}
	}
	if protocol := viper.GetString("otel-exporter-otlp-protocol"); protocol != "" {
		if err := os.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", protocol); err != nil {
			return nil, errors.Wrap(err, "setting OTEL_EXPORTER_OTLP_PROTOCOL")
		}
	}

	spanExporter, err := autoexport.NewSpanExporter(ctx)
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

	tp := newTracerProvider(spanExporter, res)
	otel.SetTracerProvider(tp)

	logExporter, err := autoexport.NewLogExporter(ctx)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, errors.Wrap(err, "creating OTLP log exporter")
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	// Bridge zap -> OTel logs so collectors receive the same records that
	// appear on stdout, correlated via requestctx.Logger's context field.
	helpers.TeeCore(otelzap.NewCore(ServiceName, otelzap.WithLoggerProvider(lp)))

	return func(ctx context.Context) error {
		var firstErr error
		if err := lp.Shutdown(ctx); err != nil {
			firstErr = err
		}
		if err := tp.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		return firstErr
	}, nil
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

// WrapHTTPRoundTripper wraps an http.RoundTripper with OpenTelemetry client
// instrumentation so outbound HTTP calls (notably Kubernetes API traffic)
// emit client spans and propagate W3C trace context. Baggage is deliberately
// excluded so caller-controlled metadata is not forwarded to the privileged
// Kubernetes API.
//
// Safe to install unconditionally: when Init installed a no-op
// TracerProvider the spans are discarded; with an exporter configured they
// are exported. Matches the Kubernetes component-base tracing pattern of
// rest.Config.Wrap(otelhttp.NewTransport).
//
// Propagators are set explicitly (not read from the global at wrap time) so
// injection works even if this runs before Init, and so a later Init cannot
// accidentally leave an already-wrapped transport on a no-op propagator.
func WrapHTTPRoundTripper(rt http.RoundTripper) http.RoundTripper {
	return otelhttp.NewTransport(rt,
		otelhttp.WithPropagators(propagation.TraceContext{}),
		otelhttp.WithSpanNameFormatter(k8sSpanName),
	)
}

// k8sSpanName derives a span name from a Kubernetes API request path, e.g.
// "GET namespaces/default/pods" or "GET nodes/my-node", instead of
// otelhttp's default of "HTTP GET" for every outbound call regardless of
// target. This makes the different Kubernetes API calls a single request
// makes distinguishable as children in a trace. Falls back to "HTTP
// {method}" for anything that doesn't look like a Kubernetes API path
// (/api/... or /apis/.../...).
func k8sSpanName(_ string, r *http.Request) string {
	fallback := "HTTP " + r.Method
	segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	var i int
	switch {
	case len(segments) >= 2 && segments[0] == "api":
		i = 2 // /api/{version}/...
	case len(segments) >= 3 && segments[0] == "apis":
		i = 3 // /apis/{group}/{version}/...
	default:
		return fallback
	}

	var resource strings.Builder
	for i < len(segments) {
		if resource.Len() > 0 {
			resource.WriteByte('/')
		}
		resource.WriteString(segments[i])
		if segments[i] == "namespaces" && i+1 < len(segments) {
			resource.WriteByte('/')
			resource.WriteString(segments[i+1])
			i += 2
			continue
		}
		i++
	}

	if resource.Len() == 0 {
		return fallback
	}

	return r.Method + " " + resource.String()
}
