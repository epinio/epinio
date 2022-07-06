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
