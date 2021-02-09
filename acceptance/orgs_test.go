package acceptance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Orgs", func() {
	Describe("create-org", func() {
		BeforeEach(func() {
			_, err := Carrier("create-org mycreatedorg", "")
			Expect(err).ToNot(HaveOccurred())
		})

		It("creates an org", func() {
			orgs, err := Carrier("orgs", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(orgs).To(MatchRegexp("mycreatedorg"))
		})

		It("targets an org", func() {
			_, err := Carrier("target mycreatedorg", "")
			Expect(err).ToNot(HaveOccurred())
			out, err := Carrier("target", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(MatchRegexp("Currently targeted organization: mycreatedorg"))
		})
	})
})
