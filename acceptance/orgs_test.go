package acceptance_test

import (
	"fmt"

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
			org := newOrgName()
			setupAndTargetOrg(org)

			By("switching org back to default")
			out, err := Epinio("target workspace", "")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("rejects creating an existing org", func() {
			org := newOrgName()
			setupAndTargetOrg(org)

			out, err := Epinio("org create "+org, "")
			Expect(err).To(HaveOccurred(), out)

			Expect(out).To(MatchRegexp(fmt.Sprintf("Organization '%s' already exists", org)))
		})
	})

	Describe("org delete", func() {
		It("deletes an org", func() {
			org := newOrgName()
			setupAndTargetOrg(org)

			By("deleting organization")
			out, err := Epinio("org delete -f "+org, "")

			Expect(err).ToNot(HaveOccurred(), out)
		})
	})
})
