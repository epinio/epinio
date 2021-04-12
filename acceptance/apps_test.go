package acceptance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps", func() {
	var org string

	BeforeEach(func() {
		org = newOrgName()
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
			out, err := Epinio("app delete "+appName, "")
			Expect(err).ToNot(HaveOccurred(), out)
			// TODO: Fix `epinio delete` from returning before the app is deleted #131

			Expect(out).To(MatchRegexp("UNBOUND SERVICES"))
			Expect(out).To(MatchRegexp(serviceName))

			Eventually(func() string {
				out, err := Epinio("app list", "")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").ShouldNot(MatchRegexp(`.*%s.*`, appName))
		})
	})

	Describe("list and show", func() {
		var appName string
		var serviceCustomName string
		BeforeEach(func() {
			appName = newAppName()
			serviceCustomName = newServiceName()
			makeApp(appName)
			makeCustomService(serviceCustomName)
			bindAppService(appName, serviceCustomName, org)
		})

		AfterEach(func() {
			deleteApp(appName)
			cleanupService(serviceCustomName)
		})

		It("lists all apps", func() {
			out, err := Epinio("app list", "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Listing applications"))
			Expect(out).To(MatchRegexp(" " + appName + " "))
			Expect(out).To(MatchRegexp(" " + serviceCustomName + " "))
		})

		It("shows the details of an app", func() {
			out, err := Epinio("app show "+appName, "")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Show application details"))
			Expect(out).To(MatchRegexp("Application: " + appName))
			Expect(out).To(MatchRegexp(`Services .*\|.* ` + serviceCustomName))
			Expect(out).To(MatchRegexp(`Routes .*\|.* ` + appName))

			Eventually(func() string {
				out, err = Epinio("app show "+appName, "")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").Should(MatchRegexp(`Status .*\|.* 1\/1`))
		})
	})
})
