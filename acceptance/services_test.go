package acceptance_test

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Services", func() {
	var namespace string
	var serviceName1 string
	var serviceName2 string
	containerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		serviceName1 = catalog.NewServiceName()
		serviceName2 = catalog.NewServiceName()
		env.SetupAndTargetNamespace(namespace)
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

	Describe("list across namespaces", func() {
		var namespace1 string
		var namespace2 string
		var service1 string
		var service2 string
		var app1 string

		// Setting up:
		// namespace1 service1 app1
		// namespace2 service1
		// namespace2 service2

		BeforeEach(func() {
			namespace1 = catalog.NewNamespaceName()
			namespace2 = catalog.NewNamespaceName()
			service1 = catalog.NewServiceName()
			service2 = catalog.NewServiceName()
			app1 = catalog.NewAppName()

			env.SetupAndTargetNamespace(namespace1)
			env.MakeService(service1)
			env.MakeContainerImageApp(app1, 1, containerImageURL)
			env.BindAppService(app1, service1, namespace1)

			env.SetupAndTargetNamespace(namespace2)
			env.MakeService(service1) // separate from namespace1.service1
			env.MakeService(service2)
		})

		AfterEach(func() {
			env.TargetNamespace(namespace2)
			env.DeleteService(service1)
			env.DeleteService(service2)

			env.TargetNamespace(namespace1)
			env.DeleteApp(app1)
			env.DeleteService(service1)
		})

		It("lists all services belonging to all namespaces", func() {
			// But we care only about the three we know about from the setup.

			out, err := env.Epinio("", "service", "list", "--all")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Listing all services"))

			Expect(out).To(MatchRegexp(fmt.Sprintf(`\| *%s *\| *%s *\| *%s *\|`, namespace1, service1, app1)))
			Expect(out).To(MatchRegexp(fmt.Sprintf(`\| *%s *\| *%s *\| *\|`, namespace2, service1)))
			Expect(out).To(MatchRegexp(fmt.Sprintf(`\| *%s *\| *%s *\| *\|`, namespace2, service2)))
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
			env.MakeContainerImageApp(appName, 1, containerImageURL)
			env.BindAppService(appName, serviceName1, namespace)

			out, err := env.Epinio("", "service", "delete", serviceName1)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Unable to delete service. It is still used by"))
			Expect(out).To(MatchRegexp(appName))
			Expect(out).To(MatchRegexp("Use --unbind to force the issue"))

			env.VerifyAppServiceBound(appName, serviceName1, namespace, 1)

			// Delete again, and force unbind

			out, err = env.Epinio("", "service", "delete", "--unbind", serviceName1)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Service Removed"))

			env.VerifyAppServiceNotbound(appName, serviceName1, namespace, 1)

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
			env.MakeContainerImageApp(appName, 1, containerImageURL)
		})

		AfterEach(func() {
			env.CleanupApp(appName)
			env.CleanupService(serviceName1)
		})

		It("binds a service to the application deployment", func() {
			env.BindAppService(appName, serviceName1, namespace)
		})
	})

	Describe("service unbind", func() {
		var appName string
		BeforeEach(func() {
			appName = catalog.NewAppName()

			env.MakeService(serviceName1)
			env.MakeContainerImageApp(appName, 1, containerImageURL)
			env.BindAppService(appName, serviceName1, namespace)
		})

		AfterEach(func() {
			env.CleanupApp(appName)
			env.CleanupService(serviceName1)
		})

		It("unbinds a service from the application deployment", func() {
			env.UnbindAppService(appName, serviceName1, namespace)
		})
	})

	Describe("service show", func() {
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

	Describe("service update", func() {
		var appName string

		BeforeEach(func() {
			appName = catalog.NewAppName()
			env.MakeContainerImageApp(appName, 1, containerImageURL)
			env.MakeService(serviceName1)
			env.BindAppService(appName, serviceName1, namespace)

			// Wait for the app restart from binding the service to settle
			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(MatchRegexp(appName + `.*\|.*1\/1.*\|.*` + serviceName1))
		})

		It("it edits the service, and restarts the app", func() {
			// edit the service ...

			out, err := env.Epinio("", "service", "update", serviceName1,
				"--remove", "username",
				"--set", "user=ci/cd",
			)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Update Service"))
			Expect(out).To(MatchRegexp(`username .*\|.* remove`))
			Expect(out).To(MatchRegexp(`user .*\|.* add/change .*\|.* ci/cd`))
			Expect(out).To(MatchRegexp("Service Changes Saved"))

			// Confirm the changes ...

			out, err = env.Epinio("", "service", "show", serviceName1)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Service Details"))
			Expect(out).To(MatchRegexp(`user .*\|.* ci/cd`))

			// Wait for app to resettle ...

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(MatchRegexp(appName + `.*\|.*1\/1.*\|.*` + serviceName1))
		})

		AfterEach(func() {
			env.TargetNamespace(namespace)
			env.DeleteApp(appName)
			env.CleanupService(serviceName1)
		})
	})
})
