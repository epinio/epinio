package acceptance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Service Classes, and Plans", func() {
	var org string
	BeforeEach(func() {
		org = newOrgName()
		setupAndTargetOrg(org)
		setupInClusterServices()
	})

	Describe("service list-classes", func() {
		It("shows all available service classes", func() {
			out, err := Epinio("service list-classes", "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("mariadb"))
			Expect(out).To(MatchRegexp("Helm Chart for mariadb"))
			Expect(out).To(MatchRegexp("minibroker"))
		})
	})

	Describe("service list-plans", func() {
		It("shows all available service plans for a class", func() {
			out, err := Epinio("service list-plans mariadb", "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("10-3-22"))
			Expect(out).To(MatchRegexp("MariaDB Server is intended"))
		})
	})
})
