// Copyright © 2021 - 2026 KRUMWARE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application_test

import (
	"context"

	"github.com/epinio/epinio/internal/application"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("StampContext", func() {
	It("keeps the stamp write alive after the operation is cancelled", func() {
		type ctxKey struct{}
		parent := context.WithValue(context.Background(), ctxKey{}, "kept")
		cancelled, cancelOp := context.WithCancel(parent)
		cancelOp()

		stampCtx, cancel := application.StampContext(cancelled)
		defer cancel()

		Expect(stampCtx.Err()).ToNot(HaveOccurred())
		Expect(stampCtx.Value(ctxKey{})).To(Equal("kept"))
	})

	It("is cancelled when its own cancel func is called", func() {
		stampCtx, cancel := application.StampContext(context.Background())
		cancel()

		Expect(stampCtx.Err()).To(MatchError(context.Canceled))
	})
})
