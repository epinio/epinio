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

package domain

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("MatchDo", func() {
	var amap DomainMap

	BeforeEach(func() {
		amap = make(DomainMap)
		amap["foxhouse"] = "dog"
		amap["*house"] = "cat"
		amap["*hou*"] = "fish"
	})

	It("returns nothing for no map", func() {
		result, err := MatchDo("anything", nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(BeEmpty())
	})

	It("matches exact before wildcard", func() {
		result, err := MatchDo("foxhouse", amap)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("dog"))
	})

	It("matches the longest wildcard pattern", func() {
		result, err := MatchDo("cathouse", amap)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("cat"))
	})

	It("matches the sole wildcard pattern", func() {
		result, err := MatchDo("houser", amap)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("fish"))
	})
})
