package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configurations", func() {
	var namespace string
	var configurationName1 string
	var configurationName2 string
	containerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		configurationName1 = catalog.NewConfigurationName()
		configurationName2 = catalog.NewConfigurationName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Describe("configuration list", func() {

		BeforeEach(func() {
			env.MakeConfiguration(configurationName1)
			env.MakeConfiguration(configurationName2)
		})

		AfterEach(func() {
			env.CleanupConfiguration(configurationName1)
			env.CleanupConfiguration(configurationName2)
		})

		It("shows all created configurations", func() {
			out, err := env.Epinio("", "configuration", "list")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(
				HaveATable(
					WithHeaders("NAME", "CREATED", "TYPE", "ORIGIN", "APPLICATIONS"),
					WithRow(configurationName1, WithDate(), "custom", "", ""),
					WithRow(configurationName2, WithDate(), "custom", "", ""),
				),
			)
		})

	})

	Describe("list across namespaces", func() {
		var namespace1 string
		var namespace2 string
		var configuration1 string
		var configuration2 string
		var app1 string

		// Setting up:
		// namespace1 configuration1 app1
		// namespace2 configuration1
		// namespace2 configuration2

		BeforeEach(func() {
			namespace1 = catalog.NewNamespaceName()
			namespace2 = catalog.NewNamespaceName()
			configuration1 = catalog.NewConfigurationName()
			configuration2 = catalog.NewConfigurationName()
			app1 = catalog.NewAppName()

			env.SetupAndTargetNamespace(namespace1)
			env.MakeConfiguration(configuration1)
			env.MakeContainerImageApp(app1, 1, containerImageURL)
			env.BindAppConfiguration(app1, configuration1, namespace1)

			env.SetupAndTargetNamespace(namespace2)
			env.MakeConfiguration(configuration1) // separate from namespace1.configuration1
			env.MakeConfiguration(configuration2)
		})

		AfterEach(func() {
			env.TargetNamespace(namespace2)
			env.DeleteConfiguration(configuration1)
			env.DeleteConfiguration(configuration2)

			env.TargetNamespace(namespace1)
			env.DeleteApp(app1)
			env.DeleteConfiguration(configuration1)

			env.DeleteNamespace(namespace1)
			env.DeleteNamespace(namespace2)
		})

		It("lists all configurations belonging to all namespaces", func() {
			// But we care only about the three we know about from the setup.

			out, err := env.Epinio("", "configuration", "list", "--all")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Listing all configurations"))
			Expect(out).To(
				HaveATable(
					WithHeaders("NAMESPACE", "NAME", "CREATED", "TYPE", "ORIGIN", "APPLICATIONS"),
					WithRow(namespace1, configuration1, WithDate(), "custom", "", app1),
					WithRow(namespace2, configuration1, WithDate(), "custom", "", ""),
					WithRow(namespace2, configuration2, WithDate(), "custom", "", ""),
				),
			)
		})
	})

	Describe("configuration create", func() {
		// Note: Configurations provision instantly.
		// No testing of wait/don't wait required.

		AfterEach(func() {
			env.CleanupConfiguration(configurationName1)
		})

		It("creates a configuration", func() {
			env.MakeConfiguration(configurationName1)
		})
	})

	Describe("configuration delete", func() {

		BeforeEach(func() {
			env.MakeConfiguration(configurationName1)
		})

		It("deletes a configuration", func() {
			env.DeleteConfiguration(configurationName1)
		})

		It("doesn't delete a bound configuration", func() {
			appName := catalog.NewAppName()
			env.MakeContainerImageApp(appName, 1, containerImageURL)
			env.BindAppConfiguration(appName, configurationName1, namespace)

			out, err := env.Epinio("", "configuration", "delete", configurationName1)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("Unable to delete configuration. It is still used by"))
			Expect(out).To(
				HaveATable(
					WithHeaders("BOUND APPLICATIONS"),
					WithRow(appName),
				),
			)
			Expect(out).To(ContainSubstring("Use --unbind to force the issue"))

			env.VerifyAppConfigurationBound(appName, configurationName1, namespace, 1)

			// Delete again, and force unbind

			out, err = env.Epinio("", "configuration", "delete", "--unbind", configurationName1)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Configuration Removed"))

			env.VerifyAppConfigurationNotbound(appName, configurationName1, namespace, 1)

			// And check non-presence
			Eventually(func() string {
				out, err = env.Epinio("", "configuration", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "2m").ShouldNot(
				HaveATable(
					WithHeaders("NAME", "CREATED", "APPLICATIONS"),
					WithRow(configurationName1, WithDate(), ""),
				),
			)
		})
	})

	Describe("configuration bind", func() {
		var appName string

		BeforeEach(func() {
			appName = catalog.NewAppName()

			env.MakeConfiguration(configurationName1)
			env.MakeContainerImageApp(appName, 1, containerImageURL)
		})

		AfterEach(func() {
			env.CleanupApp(appName)
			env.CleanupConfiguration(configurationName1)
		})

		It("binds a configuration to the application deployment", func() {
			env.BindAppConfiguration(appName, configurationName1, namespace)
		})
	})

	Describe("configuration unbind", func() {
		var appName string

		BeforeEach(func() {
			appName = catalog.NewAppName()

			env.MakeConfiguration(configurationName1)
			env.MakeContainerImageApp(appName, 1, containerImageURL)
			env.BindAppConfiguration(appName, configurationName1, namespace)
		})

		AfterEach(func() {
			env.CleanupApp(appName)
			env.CleanupConfiguration(configurationName1)
		})

		It("unbinds a configuration from the application deployment", func() {
			env.UnbindAppConfiguration(appName, configurationName1, namespace)
		})
	})

	Describe("configuration show", func() {

		BeforeEach(func() {
			env.MakeConfiguration(configurationName1)
		})

		AfterEach(func() {
			env.CleanupConfiguration(configurationName1)
		})

		It("it shows configuration details", func() {
			out, err := env.Epinio("", "configuration", "show", configurationName1)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Configuration Details"))

			Expect(out).To(
				HaveATable(
					WithHeaders("PARAMETER", "VALUE", "ACCESS PATH"),
					WithRow("username", "epinio-user", "\\/configurations\\/"+configurationName1+"\\/username"),
				),
			)
		})
	})

	Describe("configuration update", func() {
		var appName string

		BeforeEach(func() {
			appName = catalog.NewAppName()

			env.MakeContainerImageApp(appName, 1, containerImageURL)
			env.MakeConfiguration(configurationName1)
			env.BindAppConfiguration(appName, configurationName1, namespace)

			// Wait for the app restart from binding the configuration to settle
			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(
				HaveATable(
					WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(appName, WithDate(), "1/1", appName+".*", configurationName1, ""),
				),
			)
		})

		AfterEach(func() {
			env.TargetNamespace(namespace)
			env.DeleteApp(appName)
			env.CleanupConfiguration(configurationName1)
		})

		It("it edits the configuration, and restarts the app", func() {
			// edit the configuration ...

			out, err := env.Epinio("", "configuration", "update", configurationName1,
				"--remove", "username",
				"--set", "user=ci/cd",
			)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("Update Configuration"))
			Expect(out).To(
				HaveATable(
					WithHeaders("PARAMETER", "OP", "VALUE"),
					WithRow("username", "remove", ""),
					WithRow("user", "add\\/change", "ci\\/cd"),
				),
			)
			Expect(out).To(ContainSubstring("Configuration Changes Saved"))

			// Confirm the changes ...

			out, err = env.Epinio("", "configuration", "show", configurationName1)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("Configuration Details"))
			Expect(out).To(
				HaveATable(
					WithHeaders("PARAMETER", "VALUE", "ACCESS PATH"),
					WithRow("user", "ci\\/cd", "\\/configurations\\/"+configurationName1+"\\/user"),
				),
			)

			// Wait for app to resettle ...

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(
				HaveATable(
					WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(appName, WithDate(), "1/1", appName+".*", configurationName1, ""),
				),
			)
		})
	})
})
