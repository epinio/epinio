package acceptance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps", func() {
	var org = "apps-org"
	BeforeEach(func() {
		setupAndTargetOrg(org)
	})
	Describe("push and delete", func() {
		var appName string
		BeforeEach(func() {
			appName = newAppName()
		})

		It("pushes and deletes an app", func() {
			By("pushing the app")
			makeApp(appName)

			By("deleting the app")
			deleteApp(appName)
		})

		It("unbinds bound services when deleting an app", func() {
			serviceName := newServiceName()

			makeApp(appName)
			makeCustomService(serviceName)
			bindAppService(appName, serviceName, org)

			By("deleting the app")
			out, err := Carrier("delete "+appName, "")
			Expect(err).ToNot(HaveOccurred(), out)
			// TODO: Fix `carrier delete` from returning before the app is deleted #131

			Expect(out).To(MatchRegexp("Bound Services Found, Unbind Them"))
			Expect(out).To(MatchRegexp("Unbinding"))
			Expect(out).To(MatchRegexp("Service: " + serviceName))
			Expect(out).To(MatchRegexp("Unbound"))

			Eventually(func() string {
				out, err := Carrier("apps", "")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").ShouldNot(MatchRegexp(`.*%s.*`, appName))
		})
	})
})
