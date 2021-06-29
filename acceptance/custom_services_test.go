package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Custom Services", func() {
	var org string
	var serviceName string
	BeforeEach(func() {
		org = catalog.NewOrgName()
		serviceName = catalog.NewServiceName()
		env.SetupAndTargetOrg(org)
	})

	Describe("service create-custom", func() {
		// Note: Custom Services provision instantly.
		// No testing of wait/don't wait required.

		It("creates a custom service", func() {
			env.MakeCustomService(serviceName)
		})

		AfterEach(func() {
			env.CleanupService(serviceName)
		})
	})

	Describe("service delete", func() {
		BeforeEach(func() {
			env.MakeCustomService(serviceName)
		})

		It("deletes a custom service", func() {
			env.DeleteService(serviceName)
		})

		It("doesn't delete a bound service", func() {
			appName := catalog.NewAppName()
			env.MakeApp(appName, 1, true)
			env.BindAppService(appName, serviceName, org)

			out, err := env.Epinio("service delete "+serviceName, "")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Unable to delete service. It is still used by"))
			Expect(out).To(MatchRegexp(appName))
			Expect(out).To(MatchRegexp("Use --unbind to force the issue"))

			env.VerifyAppServiceBound(appName, serviceName, org, 1)

			// Delete again, and force unbind

			out, err = env.Epinio("service delete --unbind "+serviceName, "")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("PREVIOUSLY BOUND TO"))
			Expect(out).To(MatchRegexp(appName))

			Expect(out).To(MatchRegexp("Service Removed"))

			env.VerifyAppServiceNotbound(appName, serviceName, org, 1)

			// And check non-presence
			Eventually(func() string {
				out, err = env.Epinio("service list", "")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "2m").ShouldNot(MatchRegexp(serviceName))
		})
	})

	Describe("service bind", func() {
		var appName string
		BeforeEach(func() {
			appName = catalog.NewAppName()

			env.MakeCustomService(serviceName)
			env.MakeApp(appName, 1, true)
		})

		AfterEach(func() {
			env.CleanupApp(appName)
			env.CleanupService(serviceName)
		})

		It("binds a service to the application deployment", func() {
			env.BindAppService(appName, serviceName, org)
		})
	})

	Describe("service unbind", func() {
		var appName string
		BeforeEach(func() {
			appName = catalog.NewAppName()

			env.MakeCustomService(serviceName)
			env.MakeApp(appName, 1, true)
			env.BindAppService(appName, serviceName, org)
		})

		AfterEach(func() {
			env.CleanupApp(appName)
			env.CleanupService(serviceName)
		})

		It("unbinds a service from the application deployment", func() {
			env.UnbindAppService(appName, serviceName, org)
		})
	})

	Describe("service", func() {
		BeforeEach(func() {
			env.MakeCustomService(serviceName)
		})

		It("it shows service details", func() {
			out, err := env.Epinio("service show "+serviceName, "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Service Details"))
			Expect(out).To(MatchRegexp(`Status .*\|.* Provisioned`))
			Expect(out).To(MatchRegexp(`username .*\|.* epinio-user`))
		})

		AfterEach(func() {
			env.CleanupService(serviceName)
		})
	})
})
