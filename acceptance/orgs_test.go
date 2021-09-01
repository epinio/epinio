package acceptance_test

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Orgs", func() {
	It("has a default namespace", func() {
		orgs, err := env.Epinio("", "namespace", "list")
		Expect(err).ToNot(HaveOccurred())
		Expect(orgs).To(MatchRegexp("workspace"))
	})

	Describe("namespace create", func() {
		It("creates and targets an namespace", func() {
			org := catalog.NewOrgName()
			env.SetupAndTargetOrg(org)

			By("switching org back to default")
			out, err := env.Epinio("", "target", "workspace")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("rejects creating an existing namespace", func() {
			org := catalog.NewOrgName()
			env.SetupAndTargetOrg(org)

			out, err := env.Epinio("", "namespace", "create", org)
			Expect(err).To(HaveOccurred(), out)

			Expect(out).To(MatchRegexp(fmt.Sprintf("Namespace '%s' already exists", org)))
		})
	})

	Describe("namespace delete", func() {
		It("deletes an namespace", func() {
			org := catalog.NewOrgName()
			env.SetupAndTargetOrg(org)

			By("deleting namespace")
			out, err := env.Epinio("", "namespace", "delete", "-f", org)

			Expect(err).ToNot(HaveOccurred(), out)
		})
	})
})
