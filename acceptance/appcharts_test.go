package acceptance_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// [x] apps chart default
// [x] apps chart default NAME
// [x] apps chart list
// [x] apps chart show NAME
// [x] apps chart create [--short TEXT] [--desc TEXT] [--helm-repo REPO] NAME REF
// [ ] apps chart delete NAME

var _ = Describe("apps chart", func() {

	standardBall := "https://github.com/epinio/helm-charts/releases/download/epinio-application-0.1.15/epinio-application-0.1.15.tgz"

	Describe("app chart create", func() {
		When("creating a basic chart", func() {
			AfterEach(func() {
				out, err := env.Epinio("", "apps", "chart", "delete", "standard.direct")
				Expect(err).ToNot(HaveOccurred(), out)
			})
			It("creates a new chart", func() {
				out, err := env.Epinio("", "apps", "chart", "create",
					"standard.direct",
					standardBall)
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

		When("creating a described chart", func() {
			AfterEach(func() {
				out, err := env.Epinio("", "apps", "chart", "delete", "standard.direct.explained")
				Expect(err).ToNot(HaveOccurred(), out)
			})
			It("creates a new chart with descriptions", func() {
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
	})

	Describe("app chart list", func() {
		It("list the standard app chart", func() {
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
			Expect(out).To(MatchRegexp(`Helm Repository *| *https://epinio.github.io/helm-charts`))
			Expect(out).To(MatchRegexp(`Helm chart *| *epinio-application:`))
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
