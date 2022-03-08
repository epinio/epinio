package acceptance_test

import (
	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("apps env", func() {
	var (
		namespace string
		appName   string
	)

	secret := func(ns, appname string) string {
		n, err := proc.Kubectl("get", "secret", "--namespace", ns, appname+"-env", "-o", "jsonpath={.metadata.name}")
		if err != nil {
			return ""
		}
		return n
	}

	deployedEnv := func(ns, app string) string {
		out, err := proc.Kubectl("get", "deployment", "--namespace", ns, app, "-o", "jsonpath={.spec.template.spec.containers[0].env}")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		appName = catalog.NewAppName()
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Describe("app without workload", func() {
		BeforeEach(func() {
			out, err := env.Epinio("", "apps", "create", appName)
			Expect(err).ToNot(HaveOccurred(), out)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		When("unsetting an environment variable", func() {
			BeforeEach(func() {
				out, err := env.Epinio("", "apps", "env", "set", appName, "MYVAR", "myvalue")
				Expect(err).ToNot(HaveOccurred(), out)
				out, err = env.Epinio("", "apps", "env", "unset", appName, "MYVAR")
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("is not shown in the environment listing", func() {
				out, err := env.Epinio("", "apps", "env", "list", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(`MYVAR`))
				Expect(out).ToNot(ContainSubstring(`myvalue`))
			})

			It("is retrieved as empty string with show", func() {
				out, err := env.Epinio("", "apps", "env", "show", appName, "MYVAR")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(`MYVAR`)) // Var name is shown, value is empty
				Expect(out).ToNot(ContainSubstring(`myvalue`))
			})

			It("is not present in the pushed workload", func() {
				appDir := "../assets/sample-app"
				out, err := env.EpinioPush(appDir, appName, "--name", appName)
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(deployedEnv(namespace, appName)).ToNot(MatchRegexp("MYVAR"))
			})
		})

		When("setting an environment variable", func() {
			BeforeEach(func() {
				out, err := env.Epinio("", "apps", "env", "set", appName, "MYVAR", "myvalue")
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("creates the relevant secret", func() {
				secretName := secret(namespace, appName)
				Expect(secretName).ToNot(BeEmpty())
			})

			It("is shown in the environment listing", func() {
				out, err := env.Epinio("", "apps", "env", "list", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(`MYVAR`))
				Expect(out).To(ContainSubstring(`myvalue`))
			})

			It("is retrieved with show", func() {
				out, err := env.Epinio("", "apps", "env", "show", appName, "MYVAR")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(`MYVAR`))
				Expect(out).To(ContainSubstring(`myvalue`))
			})

			It("is injected into the pushed workload", func() {
				appDir := "../assets/sample-app"
				out, err := env.EpinioPush(appDir, appName, "--name", appName)
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(deployedEnv(namespace, appName)).To(MatchRegexp("MYVAR"))
			})
		})
	})

	Describe("deployed app", func() {
		BeforeEach(func() {
			appDir := "../assets/sample-app"
			out, err := env.EpinioPush(appDir, appName, "--name", appName)
			Expect(err).ToNot(HaveOccurred(), out)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		When("unsetting an environment variable", func() {
			BeforeEach(func() {
				out, err := env.Epinio("", "apps", "env", "set", appName, "MYVAR", "myvalue")
				Expect(err).ToNot(HaveOccurred(), out)

				// Wait for variable to appear so that we can verify its proper
				// removal
				Eventually(func() string {
					return deployedEnv(namespace, appName)
				}).Should(MatchRegexp("MYVAR"))
			})

			It("modifies and restarts the app", func() {
				out, err := env.Epinio("", "apps", "env", "unset", appName, "MYVAR")
				Expect(err).ToNot(HaveOccurred(), out)

				// The deployment is not expected to be immediately modified, and/or
				// the modification immediately visible. Thus waiting for the system
				// to settle here.

				Eventually(func() string {
					return deployedEnv(namespace, appName)
				}).ShouldNot(MatchRegexp("MYVAR"))
			})
		})

		When("setting an environment variable", func() {
			It("modifies and restarts the app", func() {
				out, err := env.Epinio("", "apps", "env", "set", appName, "MYVAR", "myvalue")
				Expect(err).ToNot(HaveOccurred(), out)

				// The deployment is not expected to be immediately modified, and/or
				// the modification immediately visible. Thus waiting for the system
				// to settle here.

				Eventually(func() string {
					return deployedEnv(namespace, appName)
				}).Should(MatchRegexp("MYVAR"))
			})
		})
	})
})
