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

package helpers_test

import (
	"github.com/epinio/epinio/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

var _ = Describe("TeeCore", func() {
	var prevLogger = helpers.Logger

	AfterEach(func() {
		helpers.Logger = prevLogger
	})

	It("keeps the Development/AddStacktrace behavior InitLogger configures", func() {
		Expect(helpers.InitLogger("info")).To(Succeed())

		observedCore, logs := observer.New(zapcore.WarnLevel)
		helpers.TeeCore(observedCore)

		helpers.Logger.Warn("something went wrong")

		entries := logs.TakeAll()
		Expect(entries).To(HaveLen(1))
		Expect(entries[0].Stack).NotTo(BeEmpty(),
			"a Warn-level log through the teed logger should still carry a stacktrace, "+
				"matching InitLogger's zap.NewDevelopmentConfig() behavior")
	})

	It("does not replace Logger when given a nil extra core", func() {
		Expect(helpers.InitLogger("info")).To(Succeed())
		before := helpers.Logger

		helpers.TeeCore(nil)

		Expect(helpers.Logger).To(BeIdenticalTo(before))
	})
})
