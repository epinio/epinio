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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestTracing(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tracing Suite")
}

var _ = Describe("Init", func() {
	AfterEach(func() {
		viper.Set("otel-exporter-otlp-endpoint", "")
		viper.Set("otel-exporter-otlp-protocol", "")
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
