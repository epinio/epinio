package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"
	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("apps chart", func() {
	var chartName string
	var tempFile string

	BeforeEach(func() {
		chartName = catalog.NewTmpName("chart-")
		tempFile = env.MakeAppchart(chartName)
	})
	AfterEach(func() {
		env.DeleteAppchart(tempFile)
	})

	Describe("app chart list", func() {
		It("lists the known app charts", func() {
			// These are the standard chart and a custom one with settings for the user

			out := env.Epinio("", "apps", "chart", "list")
			Expect(out).To(ContainSubstring("Show Application Charts"))

			Expect(out).To(
				HaveATable(
					WithHeaders("DEFAULT", "NAME", "CREATED", "DESCRIPTION", "#SETTINGS"),
					WithRow("standard", WithDate(), "Epinio standard deployment", "0"),
					WithRow(chartName, WithDate(), "", "9"),
				),
			)
		})
	})

	Describe("app chart show", func() {
		It("shows the details of the standard app chart", func() {
			out := env.Epinio("", "apps", "chart", "show", "standard")
			Expect(out).To(ContainSubstring("Show application chart details"))

			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Name", "standard"),
					WithRow("Created", WithDate()),
					WithRow("Short", "Epinio standard deployment"),
					WithRow("Description", "Epinio standard support chart"),
					WithRow("", "for application deployment"),
					WithRow("Helm Repository", ""),
					WithRow("Helm Chart", "https.*epinio-application.*tgz"),
				),
			)
			Expect(out).To(ContainSubstring("No settings"))
		})

		It("shows the details of the custom chart", func() {
			out := env.Epinio("", "apps", "chart", "show", chartName)
			Expect(out).To(ContainSubstring("Show application chart details"))

			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Name", chartName),
					WithRow("Created", WithDate()),
					WithRow("Short", ""),
					WithRow("Description", ""),
					WithRow("Helm Repository", ""),
					WithRow("Helm Chart", "https://github.com/epinio/helm-charts/releases/download/epinio-application-0.1.21/epinio-application-0.1.21.tgz"),
				),
			)

			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "TYPE", "ALLOWED VALUES"),
					WithRow("bar", "string", "sna, fu"),
					WithRow("cat", "number", "\\[0 ... 1]"),
					WithRow("fake", "bool", ""),
					WithRow("floof", "number", "\\[0 ... \\+inf]"),
					WithRow("foo", "string", ""),
					WithRow("fox", "integer", "\\[-inf ... 100]"),
				),
			)
		})

		It("fails to show the details of a bogus app chart", func() {
			out := env.Epinio("", "apps", "chart", "show", "bogus")
			Expect(out).To(ContainSubstring("Show application chart details"))
			Expect(out).To(ContainSubstring("Not Found: application chart 'bogus' does not exist"))
		})
	})

	Describe("app chart default", func() {
		AfterEach(func() {
			// Reset to empty default as the state to be seen at the
			// beginning of each test, regardless of ordering.
			_ = env.Epinio("", "apps", "chart", "default", "")
		})

		It("shows nothing by default", func() {
			out := env.Epinio("", "apps", "chart", "default")
			Expect(out).To(ContainSubstring("Name: not set, system default applies"))
		})

		It("sets a default", func() {
			out := env.Epinio("", "apps", "chart", "default", "standard")
			Expect(out).To(ContainSubstring("New Default Application Chart"))
			Expect(out).To(ContainSubstring("Name: standard"))

			out = env.Epinio("", "apps", "chart", "default")
			Expect(out).To(ContainSubstring("Name: standard"))
		})

		It("fails to sets a bogus default", func() {
			out := env.Epinio("", "apps", "chart", "default", "bogus")
			Expect(out).To(ContainSubstring("Not Found: application chart 'bogus' does not exist"))
		})

		It("unsets a default", func() {
			By("setting default")
			_ = env.Epinio("", "apps", "chart", "default", "standard")

			By("unsetting default")
			out := env.Epinio("", "apps", "chart", "default", "")
			Expect(out).To(ContainSubstring("Unset Default Application Chart"))

			out = env.Epinio("", "apps", "chart", "default")
			Expect(out).To(ContainSubstring("Name: not set, system default applies"))
		})
	})
})
