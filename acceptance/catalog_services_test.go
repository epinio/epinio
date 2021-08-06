package acceptance_test

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Catalog Services", func() {
	var org string
	var serviceName string
	dockerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		org = catalog.NewOrgName()
		serviceName = catalog.NewServiceName()
		env.SetupAndTargetOrg(org)
	})

	Describe("service create", func() {
		It("creates a catalog based service, with waiting", func() {
			env.MakeCatalogService(serviceName)
		})

		It("creates a catalog based service, with additional data", func() {
			env.MakeCatalogService(serviceName, `{ "db": { "name": "wordpress" }}`)
			serviceInstanceName := fmt.Sprintf("service.org-%s.svc-%s", org, serviceName)

			out, err := helpers.Kubectl("get", "serviceinstance",
				"--namespace", org, serviceInstanceName,
				"-o", "jsonpath={.status.externalProperties.parameters.db.name}")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(Equal("wordpress"))
		})

		It("creates a catalog based service, without waiting", func() {
			env.MakeCatalogServiceDontWait(serviceName)
		})

		AfterEach(func() {
			env.CleanupService(serviceName)
		})
	})

	Describe("service delete", func() {
		BeforeEach(func() {
			env.MakeCatalogService(serviceName)
		})

		It("deletes a catalog based service", func() {
			env.DeleteService(serviceName)
		})

		It("doesn't delete a bound service", func() {
			appName := catalog.NewAppName()
			env.MakeDockerImageApp(appName, 1, dockerImageURL)
			env.BindAppService(appName, serviceName, org)

			out, err := env.Epinio("", "service", "delete", serviceName)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Unable to delete service. It is still used by"))
			Expect(out).To(MatchRegexp(appName))
			Expect(out).To(MatchRegexp("Use --unbind to force the issue"))

			env.VerifyAppServiceBound(appName, serviceName, org, 1)

			// Delete again, and force unbind

			out, err = env.Epinio("", "service", "delete", "--unbind", serviceName)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("PREVIOUSLY BOUND TO"))
			Expect(out).To(MatchRegexp(appName))

			Expect(out).To(MatchRegexp("Service Removed"))

			env.VerifyAppServiceNotbound(appName, serviceName, org, 1)

			// And check non-presence
			Eventually(func() string {
				out, err = env.Epinio("", "service", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "10m").ShouldNot(MatchRegexp(serviceName))
		})
	})

	Describe("service bind", func() {
		var appName string
		BeforeEach(func() {
			appName = catalog.NewAppName()

			env.MakeCatalogService(serviceName)
			env.MakeDockerImageApp(appName, 1, dockerImageURL)
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

			env.MakeCatalogService(serviceName)
			env.MakeDockerImageApp(appName, 1, dockerImageURL)
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

	Describe("service show", func() {
		BeforeEach(func() {
			env.MakeCatalogService(serviceName)
		})

		It("it shows service details", func() {
			out, err := env.Epinio("", "service", "show", serviceName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Service Details"))
			Expect(out).To(MatchRegexp(`Status .*\|.* Provisioned`))
			Expect(out).To(MatchRegexp(`Class .*\|.* mariadb`))
			Expect(out).To(MatchRegexp(`Plan .*\|.* 10-3-22`))
		})

		AfterEach(func() {
			env.CleanupService(serviceName)
		})
	})
})
