package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bounds between Apps & Configurations", LApplication, func() {
	var namespace string
	containerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Describe("Display", func() {
		var appName string
		var configurationName string

		BeforeEach(func() {
			appName = catalog.NewAppName()
			configurationName = catalog.NewConfigurationName()

			env.MakeContainerImageApp(appName, 1, containerImageURL)
			env.MakeConfiguration(configurationName)
			env.BindAppConfiguration(appName, configurationName, namespace)
		})

		AfterEach(func() {
			// Delete app first, as this also unbinds the configuration
			env.CleanupApp(appName)
			env.CleanupConfiguration(configurationName)
		})

		It("shows the bound app for configurations list, and vice versa", func() {
			out, err := env.Epinio("", "configuration", "list")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(
				HaveATable(
					WithHeaders("NAME", "CREATED", "TYPE", "ORIGIN", "APPLICATIONS"),
					WithRow(configurationName, WithDate(), "custom", "", appName),
				),
			)

			// The next check uses `Eventually` because binding the
			// configuration to the app forces a restart of the app's
			// pod. It takes the system some time to terminate the
			// old pod, and spin up the new, during which `app list`
			// will return inconsistent results about the desired
			// and actual number of instances. We wait for the
			// system to settle back into a normal state.

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(
				HaveATable(
					WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(appName, WithDate(), "1/1", appName+".*", configurationName, ""),
				),
			)
		})
	})
})
