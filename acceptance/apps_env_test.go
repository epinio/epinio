package acceptance_test

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("apps env", func() {
	var (
		org     string
		appName string
	)

	BeforeEach(func() {
		org = catalog.NewOrgName()
		env.SetupAndTargetOrg(org)

		appName = catalog.NewAppName()
	})

	// TODO set   - deployed app restarts
	// TODO unset - not listed
	// TODO unset - not shown
	// TODO unset - not injected
	// TODO unset - deployed app restarts

	Describe("app without workload", func() {
		BeforeEach(func() {
			out, err := env.Epinio(fmt.Sprintf("apps create %s", appName), "")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		When("setting an environment variable", func() {
			BeforeEach(func() {
				out, err := env.Epinio(fmt.Sprintf("apps env set %s %s %s", appName, "MYVAR", "myvalue"), "")
				Expect(err).ToNot(HaveOccurred(), out)
			})

			secret := func(ns, appname string) string {
				n, err := helpers.Kubectl(fmt.Sprintf("get secret --namespace %s %s-env -o=jsonpath='{.metadata.name}'", ns, appname))
				if err != nil {
					return ""
				}
				return n
			}

			It("creates the relevant secret", func() {
				secretName := secret(org, appName)
				Expect(secretName).ToNot(BeEmpty())
			})

			It("is shown in the environment listing", func() {
				out, err := env.Epinio(fmt.Sprintf("apps env list %s", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(`MYVAR`))
			})

			It("is retrieved with show", func() {
				out, err := env.Epinio(fmt.Sprintf("apps env show %s MYVAR", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(`MYVAR`))
			})

			It("is injected into the pushed workload", func() {
				appDir := "../assets/sample-app"
				out, err := env.Epinio(fmt.Sprintf("apps push %s", appName), appDir)
				Expect(err).ToNot(HaveOccurred(), out)

				out, err = helpers.Kubectl(fmt.Sprintf("get deployment -n %s %s -o=jsonpath='{.spec.template.spec.containers[0].env}'", org, appName))
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp("MYVAR"))
			})
		})
	})
})
