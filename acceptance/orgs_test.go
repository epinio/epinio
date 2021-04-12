package acceptance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Orgs", func() {
	It("has a default org", func() {
		orgs, err := Epinio("org list", "")
		Expect(err).ToNot(HaveOccurred())
		Expect(orgs).To(MatchRegexp("workspace"))
	})

	Describe("org create", func() {
		It("creates and targets an org", func() {
			setupAndTargetOrg("mycreatedorg")

			By("switching org back to default")
			out, err := Epinio("target workspace", "")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("rejects creating an existing org", func() {
			setupAndTargetOrg("neworg")

			out, err := Epinio("org create neworg", "")
			Expect(err).To(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Organization 'neworg' already exists"))
		})
	})
})
