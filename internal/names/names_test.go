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

package names_test

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"

	. "github.com/epinio/epinio/internal/names"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Names", func() {
	Describe("DNSLabelSafe", func() {
		It("removes characters that are invalid in kubernetes resource names", func() {
			invalidName := "this-is.-un.a-cceptable"
			Expect(DNSLabelSafe(invalidName)).To(Equal("this-is-una-cceptable"))
		})

		It("removes leading digits", func() {
			invalidName := "123epinio-is-awesome"
			Expect(DNSLabelSafe(invalidName)).To(Equal("epinio-is-awesome"))
		})
	})

	Describe("GenerateResourceName", func() {
		It("doesn't create names longer than 63 characters", func() {
			result := GenerateResourceName("this", "is", "going", "to", "be", "too", "long", "of",
				"a", "string", "and", "will", "have", "to", "be", "truncated", "to",
				"something", "smaller")
			Expect(result).To(HaveLen(63))
		})
	})

	Describe("GenerateResourceNameTruncated", func() {
		var originalName string

		BeforeEach(func() {
			originalName = "this-is-47-characters-long-01234567890123456789"
		})

		It("doesn't create names longer than the max characters", func() {
			result := GenerateResourceNameTruncated(originalName, 42)
			Expect(result).To(HaveLen(42))
		})

		It("puts the sha1sum as a suffix", func() {
			result := GenerateResourceNameTruncated(originalName, 44)

			sumArray := sha1.Sum([]byte(originalName)) // nolint:gosec // Non-crypto use
			sum := hex.EncodeToString(sumArray[:])

			Expect(result).To(MatchRegexp(".*-%s", sum))
		})

		It("adds a prefix from the original name if enough characters", func() {
			result := GenerateResourceNameTruncated(originalName, 50)

			sumArray := sha1.Sum([]byte(originalName)) // nolint:gosec // Non-crypto use
			sum := hex.EncodeToString(sumArray[:])

			Expect(result).To(Equal(fmt.Sprintf("this-is-4-%s", sum)))
		})

		It("skips original name if maxLen is less than 42", func() {
			sumArray := sha1.Sum([]byte(originalName)) // nolint:gosec // Non-crypto use
			sum := hex.EncodeToString(sumArray[:])

			result := GenerateResourceNameTruncated(originalName, 41)
			Expect(result).To(Equal(fmt.Sprintf("x%s", sum[1:40])))

			result = GenerateResourceNameTruncated(originalName, 40)
			Expect(result).To(Equal(fmt.Sprintf("x%s", sum[1:39])))
		})
	})

	Describe("Truncate", func() {
		It("truncates the string to the desired length", func() {
			originalName := "this-is-47-characters-long-01234567890123456789"

			Expect(Truncate(originalName, 10)).To(Equal("this-is-47"))
			Expect(Truncate(originalName, 9)).To(Equal("this-is-4"))
			Expect(Truncate(originalName, 8)).To(Equal("this-is-"))
		})
	})
})
