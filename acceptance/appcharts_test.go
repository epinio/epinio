package acceptance_test

import (
	"os"
	"path"
	"path/filepath"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("apps chart", func() {

	standardBall := "https://github.com/epinio/helm-charts/releases/download/epinio-application-0.1.15/epinio-application-0.1.15.tgz"
	standardChart := "epinio-application:0.1.15"
	standardRepo := "https://epinio.github.io/helm-charts"

	Describe("app chart delete", func() {
		It("fails to delete an unknown app chart", func() {
			out, err := env.Epinio("", "apps", "chart", "delete", "bogus")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Remove application chart"))
			Expect(out).To(ContainSubstring("Not Found: Application Chart 'bogus' does not exist"))
		})

		When("deleting an existing chart chart", func() {
			BeforeEach(func() {
				out, err := env.Epinio("", "apps", "chart", "create", "to.be.deleted", standardBall)
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("deletes the chart", func() {
				out, err := env.Epinio("", "apps", "chart", "delete", "to.be.deleted")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Remove application chart"))
				Expect(out).To(ContainSubstring("Name: to.be.deleted"))
				Expect(out).To(ContainSubstring("OK"))

				out, err = env.Epinio("", "apps", "chart", "show", "to.be.deleted")
				Expect(err).To(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Show application chart details"))
				Expect(out).To(ContainSubstring("Not Found: Application Chart 'to.be.deleted' does not exist"))
			})
		})
	})

	Describe("app chart create", func() {
		When("creating a duplicate chart", func() {
			BeforeEach(func() {
				out, err := env.Epinio("", "apps", "chart", "create", "duplicate", "fox")
				Expect(err).ToNot(HaveOccurred(), out)
			})

			AfterEach(func() {
				out, err := env.Epinio("", "apps", "chart", "delete", "duplicate")
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("fails to create the chart", func() {
				out, err := env.Epinio("", "apps", "chart", "create", "duplicate", "rabbit")
				Expect(err).To(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Create Application Chart"))
				Expect(out).To(ContainSubstring("Name: duplicate"))
				Expect(out).To(ContainSubstring("Conflict: Application Chart 'duplicate' already exists"))
			})
		})

		When("creating a basic chart", func() {
			AfterEach(func() {
				out, err := env.Epinio("", "apps", "chart", "delete", "standard.direct")
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("creates a new chart", func() {
				out, err := env.Epinio("", "apps", "chart", "create", "standard.direct", standardBall)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Create Application Chart"))
				Expect(out).To(ContainSubstring("Name: standard.direct"))
				Expect(out).To(ContainSubstring("Helm Chart: " + standardBall))

				out, err = env.Epinio("", "apps", "chart", "show", "standard.direct")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Show application chart details"))
				Expect(out).To(MatchRegexp(`Key *| *VALUE`))
				Expect(out).To(MatchRegexp(`Name *| *standard.direct`))
				Expect(out).To(MatchRegexp(`Short *| *|`))
				Expect(out).To(MatchRegexp(`Description *| *|`))
				Expect(out).To(MatchRegexp(`Helm Repository *| *|`))
				Expect(out).To(MatchRegexp(`Helm chart *| *` + standardBall))
			})
		})

		When("creating a chart with descriptions", func() {
			AfterEach(func() {
				out, err := env.Epinio("", "apps", "chart", "delete", "standard.direct.explained")
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("creates a new chart with the descriptions", func() {
				out, err := env.Epinio("", "apps", "chart", "create",
					"standard.direct.explained",
					standardBall,
					"--short", "standard, direct url, described",
					"--desc", "direct url standard with descriptions",
				)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Create Application Chart"))
				Expect(out).To(ContainSubstring("Name: standard.direct"))
				Expect(out).To(ContainSubstring("Helm Chart: " + standardBall))
				Expect(out).To(ContainSubstring("Short Description: standard, direct url, described"))
				Expect(out).To(ContainSubstring("Description: direct url standard with descriptions"))

				out, err = env.Epinio("", "apps", "chart", "show", "standard.direct.explained")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Show application chart details"))
				Expect(out).To(MatchRegexp(`Key *| *VALUE`))
				Expect(out).To(MatchRegexp(`Name *| *standard.direct`))
				Expect(out).To(MatchRegexp(`Short *| *standard, direct url, described`))
				Expect(out).To(MatchRegexp(`Description *| *direct url standard with descriptions`))
				Expect(out).To(MatchRegexp(`Helm Repository *| *|`))
				Expect(out).To(MatchRegexp(`Helm chart *| *` + standardBall))
			})
		})

		When("using a chart based on repo and name+version reference", func() {
			var namespace, appName, exportPath, exportValues, exportChart, chartId string

			BeforeEach(func() {
				namespace = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespace)

				appName = catalog.NewAppName()
				chartId = catalog.NewTmpName(appName + "-chart")
				exportPath = catalog.NewTmpName(appName + "-export")
				exportValues = path.Join(exportPath, "values.yaml")
				exportChart = path.Join(exportPath, "app-chart.tar.gz")

				out, err := env.Epinio("", "apps", "chart", "create",
					chartId, standardChart, "--helm-repo", standardRepo)
				Expect(err).ToNot(HaveOccurred(), out)

				appDir := "../assets/sample-app"
				out, err = env.EpinioPush(appDir, appName,
					"--name", appName, "--app-chart", chartId)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("App is online"))
			})

			AfterEach(func() {
				out, err := env.Epinio("", "apps", "chart", "delete", chartId)
				Expect(err).ToNot(HaveOccurred(), out)

				err = os.RemoveAll(exportPath)
				Expect(err).ToNot(HaveOccurred())

				env.DeleteApp(appName)
				env.DeleteNamespace(namespace)
			})

			It("exports the chart properly from the app", func() {
				out, err := env.Epinio("", "app", "export", appName, exportPath)
				Expect(err).ToNot(HaveOccurred(), out)

				exported, err := filepath.Glob(exportPath + "/*")
				Expect(err).ToNot(HaveOccurred(), exported)
				Expect(exported).To(ConsistOf([]string{exportValues, exportChart}))

				Expect(exportPath).To(BeADirectory())
				Expect(exportValues).To(BeARegularFile())
				Expect(exportChart).To(BeARegularFile())
			})
		})
	})

	Describe("app chart list", func() {
		It("lists the known app charts", func() {
			out, err := env.Epinio("", "apps", "chart", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Show Application Charts"))
			Expect(out).To(MatchRegexp(`DEFAULT *| *NAME *| *DESCRIPTION`))
			Expect(out).To(MatchRegexp(`standard *| *Epinio standard deployment`))
		})
	})

	Describe("app chart show", func() {
		It("shows the details of standard app chart", func() {
			out, err := env.Epinio("", "apps", "chart", "show", "standard")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Show application chart details"))
			Expect(out).To(MatchRegexp(`Key *| *VALUE`))
			Expect(out).To(MatchRegexp(`Name *| *standard`))
			Expect(out).To(MatchRegexp(`Short *| *Epinio standard deployment`))
			Expect(out).To(MatchRegexp(`Description *| *Epinio standard support chart`))
			Expect(out).To(MatchRegexp(`for application deployment`))
			Expect(out).To(MatchRegexp(`Helm Repository *| *|`))
			Expect(out).To(MatchRegexp(`Helm Chart *| *epinio-application*`))
		})

		It("fails to show the details of bogus app chart", func() {
			out, err := env.Epinio("", "apps", "chart", "show", "bogus")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Show application chart details"))
			Expect(out).To(ContainSubstring("Not Found: Application Chart 'bogus' does not exist"))
		})
	})

	Describe("app chart default", func() {
		AfterEach(func() {
			// Reset to empty default as the state to be seen at the
			// beginning of each test, regardless of ordering.
			out, err := env.Epinio("", "apps", "chart", "default", "")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("shows nothing by default", func() {
			out, err := env.Epinio("", "apps", "chart", "default")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Name: not set, system default applies"))
		})

		It("sets a default", func() {
			out, err := env.Epinio("", "apps", "chart", "default", "standard")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("New Default Application Chart"))
			Expect(out).To(ContainSubstring("Name: standard"))

			out, err = env.Epinio("", "apps", "chart", "default")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Name: standard"))
		})

		It("fails to sets a bogus default", func() {
			out, err := env.Epinio("", "apps", "chart", "default", "bogus")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Not Found: Application Chart 'bogus' does not exist"))
		})

		It("unsets a default", func() {
			By("setting default")
			out, err := env.Epinio("", "apps", "chart", "default", "standard")
			Expect(err).ToNot(HaveOccurred(), out)

			By("unsetting default")
			out, err = env.Epinio("", "apps", "chart", "default", "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Unset Default Application Chart"))

			out, err = env.Epinio("", "apps", "chart", "default")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Name: not set, system default applies"))
		})
	})
})
