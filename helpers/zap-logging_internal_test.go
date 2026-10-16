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

package helpers

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var _ = Describe("InitLogger", func() {
	var prevLogger *zap.SugaredLogger
	var prevConsoleCore zapcore.Core

	BeforeEach(func() {
		prevLogger, prevConsoleCore = Logger, consoleCore
	})

	AfterEach(func() {
		Logger, consoleCore = prevLogger, prevConsoleCore
	})

	It("strips context fields on the console path when TeeCore is never called", func() {
		Expect(InitLogger("info")).To(Succeed())

		_, isStripping := Logger.Desugar().Core().(stripContextCore)
		Expect(isStripping).To(BeTrue(),
			"the console core must strip context fields even with log export "+
				"disabled, otherwise requestctx.Logger's otelzap correlation "+
				"field is printed on every request-scoped log line")
	})
})

var _ = Describe("stripContextCore", func() {
	It("drops context fields and keeps every other field", func() {
		observed, logs := observer.New(zapcore.InfoLevel)

		zap.New(stripContextCore{Core: observed}).
			With(zap.Any("context", context.Background()), zap.String("requestId", "req-1")).
			Info("hello")

		entries := logs.All()
		Expect(entries).To(HaveLen(1))

		fields := entries[0].ContextMap()
		Expect(fields).ToNot(HaveKey("context"))
		Expect(fields).To(HaveKeyWithValue("requestId", "req-1"))
	})
})
