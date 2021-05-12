package acceptance_test

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

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

		It("shows the staging logs", func() {
			By("pushing the app")
			out := makeApp(appName, 1, true)

			Expect(out).To(MatchRegexp(`.*step-create.*Configuring PHP Application.*`))
			Expect(out).To(MatchRegexp(`.*step-create.*Using feature -- PHP.*`))
		})

		It("pushes and deletes an app", func() {
			By("pushing the app in the current working directory")
			out := makeApp(appName, 1, true)

			routeRegexp := regexp.MustCompile(`https:\/\/.*omg.howdoi.website`)
			route := string(routeRegexp.Find([]byte(out)))

			resp, err := Curl("GET", route, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			By("deleting the app")
			deleteApp(appName)
		})

		It("pushes and deletes an app", func() {
			By("pushing the app in the specified app directory")
			makeApp(appName, 1, false)

			By("deleting the app")
			deleteApp(appName)
		})

		It("pushes an application with the desired number of instances", func() {
			app := newAppName()
			makeApp(app, 3, true)
			defer deleteApp(app)

			Eventually(func() string {
				out, err := Epinio("app show "+app, "")
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").Should(MatchRegexp(`Status\s*\|\s*3\/3\s*\|`))
		})

		Context("with service", func() {
			var serviceName string

			BeforeEach(func() {
				serviceName = newServiceName()
				makeCustomService(serviceName)
			})

			AfterEach(func() {
				deleteApp(appName)
				deleteService(serviceName)
			})

			It("pushes an app with bound services", func() {
				currentDir, err := os.Getwd()
				Expect(err).ToNot(HaveOccurred())

				pushOutput, err := Epinio(fmt.Sprintf("apps push %s -b %s --verbosity 1",
					appName, serviceName),
					path.Join(currentDir, "../assets/sample-app"))
				Expect(err).ToNot(HaveOccurred(), pushOutput)

				// And check presence
				Eventually(func() string {
					out, err := Epinio("app list", "")
					Expect(err).ToNot(HaveOccurred(), out)
					return out
				}, "2m").Should(MatchRegexp(appName + `.*\|.*1\/1.*\|.*` + serviceName))
			})
		})

		It("unbinds bound services when deleting an app", func() {
			serviceName := newServiceName()

			makeApp(appName, 1, true)
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

	Describe("update", func() {
		It("updates an application with the desired number of instances", func() {
			app := newAppName()
			makeApp(app, 1, true)
			defer deleteApp(app)

			Eventually(func() string {
				out, err := Epinio("app show "+app, "")
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").Should(MatchRegexp(`Status\s*\|\s*1\/1\s*\|`))

			out, err := Epinio(fmt.Sprintf("app update %s -i 3", app), "")
			Expect(err).ToNot(HaveOccurred(), out)

			Eventually(func() string {
				out, err := Epinio("app show "+app, "")
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").Should(MatchRegexp(`Status\s*\|\s*3\/3\s*\|`))
		})
	})

	Describe("list and show", func() {
		var appName string
		var serviceCustomName string
		BeforeEach(func() {
			appName = newAppName()
			serviceCustomName = newServiceName()
			makeApp(appName, 1, true)
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
