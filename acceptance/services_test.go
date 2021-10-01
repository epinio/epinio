package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Services", func() {
	var org string
	var serviceCustomName1 string
	var serviceCustomName2 string
	BeforeEach(func() {
		org = catalog.NewOrgName()
		serviceCustomName1 = catalog.NewServiceName()
		serviceCustomName2 = catalog.NewServiceName()
		env.SetupAndTargetOrg(org)
	})

	Describe("service list", func() {
		BeforeEach(func() {
			env.MakeCustomService(serviceCustomName1)
			env.MakeCustomService(serviceCustomName2)
		})

		It("shows all created services", func() {
			out, err := env.Epinio("", "service", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(serviceCustomName1))
			Expect(out).To(MatchRegexp(serviceCustomName2))
		})

		AfterEach(func() {
			env.CleanupService(serviceCustomName1)
			env.CleanupService(serviceCustomName2)
		})
	})
})
