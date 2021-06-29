package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Services", func() {
	var org string
	var serviceCatalogName string
	var serviceCustomName string
	BeforeEach(func() {
		org = catalog.NewOrgName()
		serviceCatalogName = catalog.NewServiceName()
		serviceCustomName = catalog.NewServiceName()
		env.SetupAndTargetOrg(org)
	})

	Describe("service list", func() {
		BeforeEach(func() {
			env.MakeCatalogService(serviceCatalogName)
			env.MakeCustomService(serviceCustomName)
		})

		It("shows all created services", func() {
			out, err := env.Epinio("service list", "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(serviceCustomName))
			Expect(out).To(MatchRegexp(serviceCatalogName))
		})

		AfterEach(func() {
			env.CleanupService(serviceCatalogName)
			env.CleanupService(serviceCustomName)
		})
	})
})
