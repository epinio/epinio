package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Namespaces", func() {
	It("has a default namespace", func() {
		out := env.Epinio("", "namespace", "list")
		Expect(out).To(
			HaveATable(
				WithHeaders("NAME", "CREATED", "APPLICATIONS", "CONFIGURATIONS"),
				WithRow("workspace", WithDate(), "", ""),
			),
		)
	})

	Describe("namespace create", func() {
		var namespaceName string

		BeforeEach(func() {
			namespaceName = catalog.NewNamespaceName()
		})

		AfterEach(func() {
			env.DeleteNamespace(namespaceName)
		})

		It("creates and targets an namespace", func() {
			env.SetupAndTargetNamespace(namespaceName)

			By("switching namespace back to default")
			out := env.Epinio("", "target", "workspace")
			Expect(out).To(ContainSubstring("Name: workspace"))
			Expect(out).To(ContainSubstring("Namespace targeted."))
		})

		It("rejects creating an existing namespace", func() {
			env.SetupAndTargetNamespace(namespaceName)

			out := env.EpinioErr("", "namespace", "create", namespaceName)
			Expect(out).To(ContainSubstring("namespace '%s' already exists", namespaceName))
		})
	})

	Describe("namespace list", func() {
		var namespaceName string
		var configurationName string
		var appName string

		BeforeEach(func() {
			namespaceName = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespaceName)

			configurationName = catalog.NewConfigurationName()
			env.MakeConfiguration(configurationName)

			appName = catalog.NewAppName()
			out := env.Epinio("", "app", "create", appName)
			Expect(out).To(ContainSubstring("Ok"))
		})

		AfterEach(func() {
			env.DeleteNamespace(namespaceName)
		})

		It("lists namespaces", func() {
			out := env.Epinio("", "namespace", "list", namespaceName)
			Expect(out).To(
				HaveATable(
					WithHeaders("NAME", "CREATED", "APPLICATIONS", "CONFIGURATIONS"),
					WithRow(namespaceName, WithDate(), appName, configurationName),
				),
			)
		})
	})

	Describe("namespace show", func() {
		It("rejects showing an unknown namespace", func() {
			out := env.EpinioErr("", "namespace", "show", "missing-namespace")
			Expect(out).To(ContainSubstring("namespace 'missing-namespace' does not exist"))
		})

		Context("existing namespace", func() {
			var namespaceName string
			var configurationName string
			var appName string

			BeforeEach(func() {
				namespaceName = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespaceName)

				configurationName = catalog.NewConfigurationName()
				env.MakeConfiguration(configurationName)

				appName = catalog.NewAppName()
				out := env.Epinio("", "app", "create", appName)
				Expect(out).To(ContainSubstring("Ok"))
			})

			AfterEach(func() {
				env.DeleteNamespace(namespaceName)
			})

			It("shows a namespace", func() {
				out := env.Epinio("", "namespace", "show", namespaceName)
				Expect(out).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Name", namespaceName),
						WithRow("Created", WithDate()),
						WithRow("Applications", appName),
						WithRow("Configurations", configurationName),
					),
				)
			})
		})
	})

	Describe("namespace delete", func() {
		It("deletes an namespace", func() {
			namespaceName := catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespaceName)

			By("deleting namespace")
			out := env.Epinio("", "namespace", "delete", "-f", namespaceName)
			Expect(out).To(ContainSubstring("Name: %s", namespaceName))
			Expect(out).To(ContainSubstring("Namespace deleted."))
		})
	})

	Describe("namespace target", func() {
		It("rejects targeting an unknown namespace", func() {
			out := env.EpinioErr("", "target", "missing-namespace")
			Expect(out).To(ContainSubstring("namespace 'missing-namespace' does not exist"))
		})

		Context("existing namespace", func() {
			var namespaceName string

			BeforeEach(func() {
				namespaceName = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespaceName)
			})

			AfterEach(func() {
				env.DeleteNamespace(namespaceName)
			})

			It("shows a namespace", func() {
				out := env.Epinio("", "target", namespaceName)
				Expect(out).To(ContainSubstring("Name: %s", namespaceName))
				Expect(out).To(ContainSubstring("Namespace targeted."))
			})
		})
	})
})
