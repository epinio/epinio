package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Services", func() {
	var org string
	var serviceName1 string
	var serviceName2 string
	dockerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		org = catalog.NewOrgName()
		serviceName1 = catalog.NewServiceName()
		serviceName2 = catalog.NewServiceName()
		env.SetupAndTargetOrg(org)
	})

	Describe("service list", func() {
		BeforeEach(func() {
			env.MakeService(serviceName1)
			env.MakeService(serviceName2)
		})

		It("shows all created services", func() {
			out, err := env.Epinio("", "service", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(serviceName1))
			Expect(out).To(MatchRegexp(serviceName2))
		})

		AfterEach(func() {
			env.CleanupService(serviceName1)
			env.CleanupService(serviceName2)
		})
	})

	Describe("service create", func() {
		// Note: Services provision instantly.
		// No testing of wait/don't wait required.

		It("creates a service", func() {
			env.MakeService(serviceName1)
		})

		AfterEach(func() {
			env.CleanupService(serviceName1)
		})
	})

	Describe("service delete", func() {
		BeforeEach(func() {
			env.MakeService(serviceName1)
		})

		It("deletes a service", func() {
			env.DeleteService(serviceName1)
		})

		It("doesn't delete a bound service", func() {
			appName := catalog.NewAppName()
			env.MakeDockerImageApp(appName, 1, dockerImageURL)
			env.BindAppService(appName, serviceName1, org)

			out, err := env.Epinio("", "service", "delete", serviceName1)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Unable to delete service. It is still used by"))
			Expect(out).To(MatchRegexp(appName))
			Expect(out).To(MatchRegexp("Use --unbind to force the issue"))

			env.VerifyAppServiceBound(appName, serviceName1, org, 1)

			// Delete again, and force unbind

			out, err = env.Epinio("", "service", "delete", "--unbind", serviceName1)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Service Removed"))

			env.VerifyAppServiceNotbound(appName, serviceName1, org, 1)

			// And check non-presence
			Eventually(func() string {
				out, err = env.Epinio("", "service", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "2m").ShouldNot(MatchRegexp(serviceName1))
		})
	})

	Describe("service bind", func() {
		var appName string
		BeforeEach(func() {
			appName = catalog.NewAppName()

			env.MakeService(serviceName1)
			env.MakeDockerImageApp(appName, 1, dockerImageURL)
		})

		AfterEach(func() {
			env.CleanupApp(appName)
			env.CleanupService(serviceName1)
		})

		It("binds a service to the application deployment", func() {
			env.BindAppService(appName, serviceName1, org)
		})
	})

	Describe("service unbind", func() {
		var appName string
		BeforeEach(func() {
			appName = catalog.NewAppName()

			env.MakeService(serviceName1)
			env.MakeDockerImageApp(appName, 1, dockerImageURL)
			env.BindAppService(appName, serviceName1, org)
		})

		AfterEach(func() {
			env.CleanupApp(appName)
			env.CleanupService(serviceName1)
		})

		It("unbinds a service from the application deployment", func() {
			env.UnbindAppService(appName, serviceName1, org)
		})
	})

	Describe("service", func() {
		BeforeEach(func() {
			env.MakeService(serviceName1)
		})

		It("it shows service details", func() {
			out, err := env.Epinio("", "service", "show", serviceName1)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Service Details"))
			Expect(out).To(MatchRegexp(`username .*\|.* epinio-user`))
		})

		AfterEach(func() {
			env.CleanupService(serviceName1)
		})
	})
})
