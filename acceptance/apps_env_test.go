package acceptance_test

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("apps env", func() {
	var (
		namespace string
		appName   string
	)

	secret := func(ns, appname string) string {
		n, err := proc.Kubectl("get", "secret",
			"--namespace", ns,
			"-l", fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s,epinio.io/area=environment", appname, ns),
			"-o", "jsonpath={.items[].metadata.name")
		if err != nil {
			return err.Error()
		}
		return n
	}

	deployedEnv := func(ns, app string) string {
		out, err := proc.Kubectl("get", "deployments",
			"-l", fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s", app, ns),
			"--namespace", ns,
			"-o", "jsonpath={.items[].spec.template.spec.containers[0].env}")

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
			_ = env.Epinio("", "apps", "create", appName)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		When("unsetting an environment variable", func() {
			BeforeEach(func() {
				_ = env.Epinio("", "apps", "env", "set", appName, "MYVAR", "myvalue")
				_ = env.Epinio("", "apps", "env", "unset", appName, "MYVAR")
			})

			It("is not shown in the environment listing", func() {
				out := env.Epinio("", "apps", "env", "list", appName)

				Expect(out).To(HaveATable(WithHeaders("VARIABLE", "VALUE")))
				Expect(out).ToNot(HaveATable(WithRow("MYVAR", "myvalue")))
			})

			It("is retrieved as empty string with show", func() {
				out := env.Epinio("", "apps", "env", "show", appName, "MYVAR")

				Expect(out).To(ContainSubstring(`Variable: MYVAR`)) // Var name is shown, value is empty
				Expect(out).ToNot(ContainSubstring(`myvalue`))
			})

			It("is not present in the pushed workload", func() {
				appDir := "../assets/sample-app"
				out, err := env.EpinioPush(appDir, appName, "--name", appName)
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(deployedEnv(namespace, appName)).ToNot(ContainSubstring("MYVAR"))
			})
		})

		When("setting an environment variable", func() {
			BeforeEach(func() {
				_ = env.Epinio("", "apps", "env", "set", appName, "MYVAR", "myvalue")
			})

			It("creates the relevant secret", func() {
				secretName := secret(namespace, appName)
				Expect(secretName).ToNot(BeEmpty())
			})

			It("is shown in the environment listing", func() {
				out := env.Epinio("", "apps", "env", "list", appName)

				Expect(out).To(
					HaveATable(
						WithHeaders("VARIABLE", "VALUE"),
						WithRow("MYVAR", "myvalue"),
					),
				)
			})

			It("is retrieved with show", func() {
				out := env.Epinio("", "apps", "env", "show", appName, "MYVAR")
				Expect(out).To(ContainSubstring(`Variable: MYVAR`))
				Expect(out).To(ContainSubstring(`Value: myvalue`))
			})

			It("is injected into the pushed workload", func() {
				appDir := "../assets/sample-app"
				out, err := env.EpinioPush(appDir, appName, "--name", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(deployedEnv(namespace, appName)).To(ContainSubstring("MYVAR"))
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
				_ = env.Epinio("", "apps", "env", "set", appName, "MYVAR", "myvalue")

				// Wait for variable to appear so that we can verify its proper
				// removal
				Eventually(func() string {
					return deployedEnv(namespace, appName)
				}).Should(ContainSubstring("MYVAR"))
			})

			It("modifies and restarts the app", func() {
				_ = env.Epinio("", "apps", "env", "unset", appName, "MYVAR")

				// The deployment is not expected to be immediately modified, and/or
				// the modification immediately visible. Thus waiting for the system
				// to settle here.

				Eventually(func() string {
					return deployedEnv(namespace, appName)
				}).ShouldNot(ContainSubstring("MYVAR"))
			})
		})

		When("setting an environment variable", func() {
			It("modifies and restarts the app", func() {
				_ = env.Epinio("", "apps", "env", "set", appName, "MYVAR", "myvalue")

				// The deployment is not expected to be immediately modified, and/or
				// the modification immediately visible. Thus waiting for the system
				// to settle here.

				Eventually(func() string {
					return deployedEnv(namespace, appName)
				}).Should(ContainSubstring("MYVAR"))
			})
		})
	})
})
