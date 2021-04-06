package acceptance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Orgs", func() {
	It("has a default org", func() {
		orgs, err := Carrier("org list", "")
		Expect(err).ToNot(HaveOccurred())
		Expect(orgs).To(MatchRegexp("workspace"))
	})

	Describe("create-org", func() {
		It("creates and targets an org", func() {
			setupAndTargetOrg("mycreatedorg")

			By("switching org back to default")
			out, err := Carrier("target workspace", "")
			Expect(err).ToNot(HaveOccurred(), out)
		})
	})
})
