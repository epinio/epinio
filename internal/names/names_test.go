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
		var invalidName string

		BeforeEach(func() {
			invalidName = "this-is.-un.a-cceptable"
		})

		It("removes characters that are invalid in kubernetes resource names", func() {
			Expect(DNSLabelSafe(invalidName)).To(Equal("this-is-una-cceptable"))
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
			Expect(result).To(Equal(sum))

			result = GenerateResourceNameTruncated(originalName, 40)
			Expect(result).To(Equal(sum))
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
