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

package tracing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/epinio/epinio/helpers"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestTracing(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tracing Suite")
}

func preserveEnv(name string) {
	value, exists := os.LookupEnv(name)
	DeferCleanup(func() {
		if exists {
			Expect(os.Setenv(name, value)).To(Succeed())
		} else {
			Expect(os.Unsetenv(name)).To(Succeed())
		}
	})
}

var _ = Describe("Init", func() {
	BeforeEach(func() {
		for _, name := range []string{
			"OTEL_EXPORTER_OTLP_ENDPOINT",
			"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
			"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT",
			"OTEL_EXPORTER_OTLP_PROTOCOL",
		} {
			preserveEnv(name)
			Expect(os.Unsetenv(name)).To(Succeed())
		}
	})

	AfterEach(func() {
		viper.Set("otel-exporter-otlp-endpoint", "")
		viper.Set("otel-exporter-otlp-protocol", "")
		otel.SetTracerProvider(noop.NewTracerProvider())
	})

	When("no endpoint is configured", func() {
		It("installs a no-op tracer provider and a working shutdown func", func() {
			viper.Set("otel-exporter-otlp-endpoint", "")

			shutdown, err := Init(context.Background())
			Expect(err).ToNot(HaveOccurred())
			Expect(shutdown).ToNot(BeNil())

			_, isNoop := otel.GetTracerProvider().(noop.TracerProvider)
			Expect(isNoop).To(BeTrue())

			// Init sets the propagator unconditionally, even on the
			// no-op path - check it carries both TraceContext and
			// Baggage fields via the header names each contributes.
			Expect(otel.GetTextMapPropagator().Fields()).To(ContainElements(
				"traceparent", "baggage",
			))

			Expect(shutdown(context.Background())).To(Succeed())
		})
	})

	When("only the signal-specific traces endpoint is configured", func() {
		It("enables tracing without enabling log export", func() {
			Expect(os.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://127.0.0.1:4318")).To(Succeed())
			viper.Set("otel-exporter-otlp-endpoint", "")
			viper.Set("otel-exporter-otlp-protocol", "http/protobuf")
			loggerBeforeInit := helpers.Logger

			shutdown, err := Init(context.Background())
			Expect(err).ToNot(HaveOccurred())

			_, isSDKProvider := otel.GetTracerProvider().(*sdktrace.TracerProvider)
			Expect(isSDKProvider).To(BeTrue())
			Expect(helpers.Logger).To(BeIdenticalTo(loggerBeforeInit))
			Expect(shutdown(context.Background())).To(Succeed())
		})
	})

	When("only the signal-specific logs endpoint is configured", func() {
		It("enables log export without enabling tracing", func() {
			Expect(os.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "http://127.0.0.1:4318")).To(Succeed())
			viper.Set("otel-exporter-otlp-endpoint", "")
			viper.Set("otel-exporter-otlp-protocol", "http/protobuf")
			loggerBeforeInit := helpers.Logger
			DeferCleanup(func() {
				helpers.Logger = loggerBeforeInit
			})

			shutdown, err := Init(context.Background())
			Expect(err).ToNot(HaveOccurred())

			_, isNoop := otel.GetTracerProvider().(noop.TracerProvider)
			Expect(isNoop).To(BeTrue())
			Expect(helpers.Logger).ToNot(BeIdenticalTo(loggerBeforeInit))
			Expect(shutdown(context.Background())).To(Succeed())
		})
	})
})

var _ = Describe("newTracerProvider", func() {
	It("records spans with the configured resource and flushes them on shutdown", func() {
		exporter := tracetest.NewInMemoryExporter()
		res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
			semconv.ServiceName(ServiceName),
			semconv.ServiceVersion("v1.2.3"),
		))
		Expect(err).ToNot(HaveOccurred())

		tp := newTracerProvider(exporter, res)

		_, span := tp.Tracer("test").Start(context.Background(), "test-span")
		span.End()

		// ForceFlush drains the batch processor's queue into the exporter
		// without clearing it, unlike Shutdown (which also shuts down and
		// resets the in-memory exporter itself) - so spans must be read here.
		Expect(tp.ForceFlush(context.Background())).To(Succeed())

		spans := exporter.GetSpans()
		Expect(spans).To(HaveLen(1))
		Expect(spans[0].Name).To(Equal("test-span"))

		attrs := spans[0].Resource.Attributes()
		Expect(attrs).To(ContainElement(semconv.ServiceName(ServiceName)))
		Expect(attrs).To(ContainElement(semconv.ServiceVersion("v1.2.3")))

		Expect(tp.Shutdown(context.Background())).To(Succeed())
	})
})

var _ = Describe("WrapHTTPRoundTripper", func() {
	It("records a client span and injects trace context without forwarding baggage", func() {
		exporter := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
		prev := otel.GetTracerProvider()
		otel.SetTracerProvider(tp)
		DeferCleanup(func() {
			otel.SetTracerProvider(prev)
			_ = tp.Shutdown(context.Background())
		})

		var gotTraceparent string
		var gotBaggage string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotTraceparent = r.Header.Get("traceparent")
			gotBaggage = r.Header.Get("baggage")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))
		DeferCleanup(server.Close)

		client := &http.Client{Transport: WrapHTTPRoundTripper(http.DefaultTransport)}
		ctx, parent := tp.Tracer("test").Start(context.Background(), "parent")
		member, err := baggage.NewMember("untrusted", "caller-controlled")
		Expect(err).ToNot(HaveOccurred())
		bag, err := baggage.New(member)
		Expect(err).ToNot(HaveOccurred())
		ctx = baggage.ContextWithBaggage(ctx, bag)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/namespaces", nil)
		Expect(err).ToNot(HaveOccurred())

		resp, err := client.Do(req)
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() { _ = resp.Body.Close() })
		_, _ = io.Copy(io.Discard, resp.Body)
		parent.End()

		Expect(gotTraceparent).ToNot(BeEmpty())
		Expect(gotBaggage).To(BeEmpty())

		spans := exporter.GetSpans()
		Expect(len(spans)).To(BeNumerically(">=", 1))

		var clientSpan tracetest.SpanStub
		found := false
		for _, s := range spans {
			if s.SpanKind == trace.SpanKindClient {
				clientSpan = s
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "expected a client span from otelhttp")
		Expect(clientSpan.Parent.SpanID()).To(Equal(parent.SpanContext().SpanID()))
	})
})
