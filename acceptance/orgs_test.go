package acceptance_test

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Orgs", func() {
	It("has a default org", func() {
		orgs, err := env.Epinio("org list", "")
		Expect(err).ToNot(HaveOccurred())
		Expect(orgs).To(MatchRegexp("workspace"))
	})

	Describe("org create", func() {
		It("creates and targets an org", func() {
			org := catalog.NewOrgName()
			env.SetupAndTargetOrg(org)

			By("switching org back to default")
			out, err := env.Epinio("target workspace", "")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("rejects creating an existing org", func() {
			org := catalog.NewOrgName()
			env.SetupAndTargetOrg(org)

			out, err := env.Epinio("org create "+org, "")
			Expect(err).To(HaveOccurred(), out)

			Expect(out).To(MatchRegexp(fmt.Sprintf("Organization '%s' already exists", org)))
		})
	})

	Describe("org delete", func() {
		It("deletes an org", func() {
			org := catalog.NewOrgName()
			env.SetupAndTargetOrg(org)

			By("deleting organization")
			out, err := env.Epinio("org delete -f "+org, "")

			Expect(err).ToNot(HaveOccurred(), out)
		})
	})
})
