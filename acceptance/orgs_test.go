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

	Describe("namespace list", func() {
		var org string
		var serviceName string
		var appName string

		BeforeEach(func() {
			org = catalog.NewOrgName()
			env.SetupAndTargetOrg(org)

			serviceName = catalog.NewServiceName()
			env.MakeService(serviceName)

			appName = catalog.NewAppName()
			out, err := env.Epinio("", "app", "create", appName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Ok"))
		})

		It("lists namespaces", func() {
			out, err := env.Epinio("", "namespace", "list", org)

			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(fmt.Sprintf(`%s.* \| .*%s.* \| .*%s`, org, appName, serviceName)))
		})
	})

	Describe("namespace show", func() {
		It("rejects showing an unknown namespace", func() {
			out, err := env.Epinio("", "namespace", "show", "missing-namespace")
			Expect(err).To(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("namespace 'missing-namespace' does not exist"))
		})

		Context("existing namespace", func() {
			var org string
			var serviceName string
			var appName string

			BeforeEach(func() {
				org = catalog.NewOrgName()
				env.SetupAndTargetOrg(org)

				serviceName = catalog.NewServiceName()
				env.MakeService(serviceName)

				appName = catalog.NewAppName()
				out, err := env.Epinio("", "app", "create", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp("Ok"))
			})

			It("shows a namespace", func() {
				out, err := env.Epinio("", "namespace", "show", org)

				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp(fmt.Sprintf(`Name .*\| .*%s`, org)))
				Expect(out).To(MatchRegexp(fmt.Sprintf(`Services .*\| .*%s`, serviceName)))
				Expect(out).To(MatchRegexp(fmt.Sprintf(`Applications .*\| .*%s`, appName)))
			})
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
