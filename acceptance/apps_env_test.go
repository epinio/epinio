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

	secret := func(ns, appname string) string {
		n, err := helpers.Kubectl(fmt.Sprintf("get secret --namespace %s %s-env -o=jsonpath='{.metadata.name}'",
			ns, appname))
		if err != nil {
			return ""
		}
		return n
	}

	deployedEnv := func(ns, app string) string {
		out, err := helpers.Kubectl(
			fmt.Sprintf("get deployment --namespace %s %s -o=jsonpath='{.spec.template.spec.containers[0].env}'",
				ns, app))
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}

	BeforeEach(func() {
		org = catalog.NewOrgName()
		env.SetupAndTargetOrg(org)

		appName = catalog.NewAppName()
	})

	Describe("app without workload", func() {
		BeforeEach(func() {
			out, err := env.Epinio(fmt.Sprintf("apps create %s", appName), "")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		When("unsetting an environment variable", func() {
			BeforeEach(func() {
				out, err := env.Epinio(fmt.Sprintf("apps env set %s MYVAR myvalue", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)
				out, err = env.Epinio(fmt.Sprintf("apps env unset %s MYVAR", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("is not shown in the environment listing", func() {
				out, err := env.Epinio(fmt.Sprintf("apps env list %s", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(`MYVAR`))
				Expect(out).ToNot(ContainSubstring(`myvalue`))
			})

			It("is retrieved as empty string with show", func() {
				out, err := env.Epinio(fmt.Sprintf("apps env show %s MYVAR", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(`MYVAR`)) // Var name ois shown, value is empty
				Expect(out).ToNot(ContainSubstring(`myvalue`))
			})

			It("is not present in the pushed workload", func() {
				appDir := "../assets/sample-app"
				out, err := env.Epinio(fmt.Sprintf("apps push %s", appName), appDir)
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(deployedEnv(org, appName)).ToNot(MatchRegexp("MYVAR"))
			})
		})

		When("setting an environment variable", func() {
			BeforeEach(func() {
				out, err := env.Epinio(fmt.Sprintf("apps env set %s MYVAR myvalue", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("creates the relevant secret", func() {
				secretName := secret(org, appName)
				Expect(secretName).ToNot(BeEmpty())
			})

			It("is shown in the environment listing", func() {
				out, err := env.Epinio(fmt.Sprintf("apps env list %s", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(`MYVAR`))
				Expect(out).To(ContainSubstring(`myvalue`))
			})

			It("is retrieved with show", func() {
				out, err := env.Epinio(fmt.Sprintf("apps env show %s MYVAR", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(`MYVAR`))
				Expect(out).To(ContainSubstring(`myvalue`))
			})

			It("is injected into the pushed workload", func() {
				appDir := "../assets/sample-app"
				out, err := env.Epinio(fmt.Sprintf("apps push %s", appName), appDir)
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(deployedEnv(org, appName)).To(MatchRegexp("MYVAR"))
			})
		})
	})

	Describe("deployed app", func() {
		BeforeEach(func() {
			appDir := "../assets/sample-app"
			out, err := env.Epinio(fmt.Sprintf("apps push %s", appName), appDir)
			Expect(err).ToNot(HaveOccurred(), out)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		When("unsetting an environment variable", func() {
			BeforeEach(func() {
				out, err := env.Epinio(fmt.Sprintf("apps env set %s MYVAR myvalue", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)

				// Wait for variable to appear so that we can verify its proper
				// removal
				Eventually(func() string {
					return deployedEnv(org, appName)
				}).Should(MatchRegexp("MYVAR"))
			})

			It("modifies and restarts the app", func() {
				out, err := env.Epinio(fmt.Sprintf("apps env unset %s MYVAR", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)

				// The deployment is not expected to be immediately modified, and/or
				// the modification immediately visible. Thus waiting for the system
				// to settle here.

				Eventually(func() string {
					return deployedEnv(org, appName)
				}).ShouldNot(MatchRegexp("MYVAR"))
			})
		})

		When("setting an environment variable", func() {
			It("modifies and restarts the app", func() {
				out, err := env.Epinio(fmt.Sprintf("apps env set %s MYVAR myvalue", appName), "")
				Expect(err).ToNot(HaveOccurred(), out)

				// The deployment is not expected to be immediately modified, and/or
				// the modification immediately visible. Thus waiting for the system
				// to settle here.

				Eventually(func() string {
					return deployedEnv(org, appName)
				}).Should(MatchRegexp("MYVAR"))
			})
		})
	})
})
