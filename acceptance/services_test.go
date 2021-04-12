package acceptance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Services", func() {
	var org string
	var serviceCatalogName string
	var serviceCustomName string
	BeforeEach(func() {
		org = newOrgName()
		serviceCatalogName = newServiceName()
		serviceCustomName = newServiceName()
		setupAndTargetOrg(org)
		setupInClusterServices()
	})

	Describe("service list", func() {
		BeforeEach(func() {
			makeCatalogService(serviceCatalogName)
			makeCustomService(serviceCustomName)
		})

		It("shows all created services", func() {
			out, err := Epinio("service list", "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(serviceCustomName))
			Expect(out).To(MatchRegexp(serviceCatalogName))
		})

		AfterEach(func() {
			cleanupService(serviceCatalogName)
			cleanupService(serviceCustomName)
		})
	})
})
