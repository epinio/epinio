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

package cahash_test

import (
	"github.com/epinio/epinio/helpers/cahash"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CA Hash CanonicalString", func() {
	It("properly transforms the string", func() {
		r := cahash.CanonicalString(" \f\n \v\thello      \t \f \n \v  world\t \v\n \f ")
		Expect(r).To(Equal("hello world"))
	})
})
