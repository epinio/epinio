package acceptance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Orgs", func() {
	It("has a default org", func() {
		orgs, err := Carrier("orgs", "")
		Expect(err).ToNot(HaveOccurred())
		Expect(orgs).To(MatchRegexp("workspace"))
	})

	Describe("create-org", func() {
		It("creates and targets an org", func() {
			By("creating an org")
			_, err := Carrier("create-org mycreatedorg", "")
			Expect(err).ToNot(HaveOccurred())
			orgs, err := Carrier("orgs", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(orgs).To(MatchRegexp("mycreatedorg"))

			By("targeting an org")
			_, err = Carrier("target mycreatedorg", "")
			Expect(err).ToNot(HaveOccurred())
			out, err := Carrier("target", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(MatchRegexp("Currently targeted organization: mycreatedorg"))
		})
	})
})
