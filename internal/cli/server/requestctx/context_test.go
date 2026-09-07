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

package requestctx_test

import (
	"context"
	"testing"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestctx(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Requestctx Suite")
}

var _ = Describe("Logger", func() {
	var (
		observed *observer.ObservedLogs
		prev     *zap.SugaredLogger
	)

	BeforeEach(func() {
		prev = helpers.Logger
		core, logs := observer.New(zapcore.InfoLevel)
		helpers.Logger = zap.New(core).Sugar()
		observed = logs
	})

	AfterEach(func() {
		helpers.Logger = prev
	})

	It("adds requestId when present", func() {
		ctx := requestctx.WithID(context.Background(), "req-123")
		requestctx.Logger(ctx).Infow("hello")

		entries := observed.All()
		Expect(entries).To(HaveLen(1))
		Expect(entries[0].ContextMap()).To(HaveKeyWithValue("requestId", "req-123"))
	})

	It("adds trace_id and span_id from a valid span context", func() {
		traceID, err := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
		Expect(err).ToNot(HaveOccurred())
		spanID, err := trace.SpanIDFromHex("0102030405060708")
		Expect(err).ToNot(HaveOccurred())

		sc := trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    traceID,
			SpanID:     spanID,
			TraceFlags: trace.FlagsSampled,
		})
		ctx := trace.ContextWithSpanContext(context.Background(), sc)
		ctx = requestctx.WithID(ctx, "req-456")

		requestctx.Logger(ctx).Infow("traced")

		entries := observed.All()
		Expect(entries).To(HaveLen(1))
		m := entries[0].ContextMap()
		Expect(m).To(HaveKeyWithValue("requestId", "req-456"))
		Expect(m).To(HaveKeyWithValue("trace_id", traceID.String()))
		Expect(m).To(HaveKeyWithValue("span_id", spanID.String()))
	})

	It("returns the base logger when context has no request id or span", func() {
		requestctx.Logger(context.Background()).Infow("plain")

		entries := observed.All()
		Expect(entries).To(HaveLen(1))
		Expect(entries[0].ContextMap()).NotTo(HaveKey("trace_id"))
		Expect(entries[0].ContextMap()).NotTo(HaveKey("requestId"))
	})
})
