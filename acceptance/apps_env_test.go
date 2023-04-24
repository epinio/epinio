// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package acceptance_test

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("apps env", LApplication, func() {
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

				Expect(out).To(HaveATable(WithHeaders("VARIABLE", "VALUE")))
				Expect(out).ToNot(HaveATable(WithRow("MYVAR", "myvalue")))
			})

			It("is retrieved as empty string with show", func() {
				out, err := env.Epinio("", "apps", "env", "show", appName, "MYVAR")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(`Variable: MYVAR`)) // Var name is shown, value is empty
				Expect(out).ToNot(ContainSubstring(`myvalue`))
			})

			It("is not present in the pushed workload", func() {
				appDir := "../assets/sample-app"
				out, err := env.EpinioPush(appDir, appName, "--name", appName)
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(deployedEnv(namespace, appName)).ToNot(ContainSubstring("MYVAR"))
			})

			Context("command completion", func() {
				// MYVAX is intentionally different from MYVAR, to avoid possible clashes
				BeforeEach(func() {
					out, err := env.Epinio("", "apps", "env", "set", appName, "MYVAX", "myvalue")
					Expect(err).ToNot(HaveOccurred(), out)
				})

				AfterEach(func() {
					out, err := env.Epinio("", "apps", "env", "unset", appName, "MYVAX")
					Expect(err).ToNot(HaveOccurred(), out)
				})

				It("matches empty app prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "unset", "")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).To(ContainSubstring(appName))
				})

				It("matches empty var prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "unset", appName, "")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).To(ContainSubstring("MYVAX"))
				})

				It("does not match unknown app prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "unset", "bogus")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(ContainSubstring("bogus"))
				})

				It("does not match unknown var prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "unset", appName, "bogus")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(ContainSubstring("bogus"))
				})

				It("does not match bogus arguments", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "unset", appName, "MYVAX", "")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(ContainSubstring(appName))
					Expect(out).ToNot(ContainSubstring("MYVAX"))
				})
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

				Expect(out).To(
					HaveATable(
						WithHeaders("VARIABLE", "VALUE"),
						WithRow("MYVAR", "myvalue"),
					),
				)
			})

			Context("list command completion", func() {
				It("matches empty prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "list", "")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).To(ContainSubstring(appName))
				})

				It("does not match unknown prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "list", "bogus")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(ContainSubstring("bogus"))
				})

				It("does not match bogus arguments", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "list", appName, "")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(ContainSubstring(appName))
					Expect(out).ToNot(ContainSubstring("MYVAR"))
				})
			})

			It("is retrieved with show", func() {
				out, err := env.Epinio("", "apps", "env", "show", appName, "MYVAR")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(`Variable: MYVAR`))
				Expect(out).To(ContainSubstring(`Value: myvalue`))
			})

			Context("show command completion", func() {
				It("matches empty app prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "show", "")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).To(ContainSubstring(appName))
				})

				It("matches empty var prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "show", appName, "")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).To(ContainSubstring("MYVAR"))
				})

				It("does not match unknown app prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "show", "bogus")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(ContainSubstring("bogus"))
				})

				It("does not match unknown var prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "show", appName, "bogus")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(ContainSubstring("bogus"))
				})

				It("does not match bogus arguments", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "show", appName, "MYVAR", "")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(ContainSubstring(appName))
					Expect(out).ToNot(ContainSubstring("MYVAR"))
				})
			})

			It("is injected into the pushed workload", func() {
				appDir := "../assets/sample-app"
				out, err := env.EpinioPush(appDir, appName, "--name", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(deployedEnv(namespace, appName)).To(ContainSubstring("MYVAR"))
			})

			Context("command completion", func() {
				It("matches empty prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "set", "")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).To(ContainSubstring(appName))
				})

				It("does not match unknown prefix", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "set", "bogus")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(ContainSubstring("bogus"))
				})

				It("does not match bogus arguments", func() {
					out, err := env.Epinio("", "__complete", "app", "env", "set", appName, "")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(ContainSubstring(appName))
					Expect(out).ToNot(ContainSubstring("MYVAR"))
				})
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
				}).Should(ContainSubstring("MYVAR"))
			})

			It("modifies and restarts the app", func() {
				out, err := env.Epinio("", "apps", "env", "unset", appName, "MYVAR")
				Expect(err).ToNot(HaveOccurred(), out)

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
				out, err := env.Epinio("", "apps", "env", "set", appName, "MYVAR", "myvalue")
				Expect(err).ToNot(HaveOccurred(), out)

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
