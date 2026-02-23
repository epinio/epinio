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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configurations", LConfiguration, func() {
	var namespace string
	var configurationName1 string
	var configurationName2 string
	containerImageURL := "epinio/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		configurationName1 = catalog.NewConfigurationName()
		configurationName2 = catalog.NewConfigurationName()
		env.SetupAndTargetNamespace(namespace)

		DeferCleanup(func() {
			env.DeleteNamespace(namespace)
		})
	})

	Describe("configuration list", func() {

		BeforeEach(func() {
			env.MakeConfiguration(configurationName1)
			env.MakeConfiguration(configurationName2)
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

		It("lists configurations in JSON format", func() {
			out, err := env.Epinio("", "configuration", "list", "--output", "json")
			Expect(err).ToNot(HaveOccurred(), out)

			configurations := models.ConfigurationResponseList{}
			err = json.Unmarshal([]byte(extractJSONPayload(out)), &configurations)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(configurations).ToNot(BeEmpty())
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

			DeferCleanup(func() {
				env.DeleteNamespace(namespace1)
				env.DeleteNamespace(namespace2)
			})
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

		It("creates a configuration", func() {
			env.MakeConfiguration(configurationName1)
		})

		It("creates an empty configuration", func() {
			out, err := env.Epinio("", "configuration", "create", configurationName1)
			Expect(err).ToNot(HaveOccurred(), out)

			// Check presence
			out, err = env.Epinio("", "configuration", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(configurationName1))

			// No parameter
			out, err = env.Epinio("", "configuration", "show", configurationName1)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Configuration Details"))
			Expect(out).To(ContainSubstring("No parameters"))
		})
	})

	Describe("configuration create failures", func() {
		It("rejects names not fitting kubernetes requirements", func() {
			out, err := env.Epinio("", "configuration", "create", "BOGUS", "dummy", "value")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("name must consist of lower case alphanumeric"))
		})

		It("fails for missing arguments, not enough, no files", func() {
			out, err := env.Epinio("", "configuration", "create")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Not enough arguments, expected name"))
		})

		It("fails for missing arguments, not enough, with files", func() {
			out, err := env.Epinio("", "configuration", "create", "--from-file", "dummy")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Not enough arguments, expected name"))
		})

		It("fails for missing arguments, key without value", func() {
			out, err := env.Epinio("", "configuration", "create", "foo", "a")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Last Key has no value"))
		})

		It("fails for a missing path", func() {
			out, err := env.Epinio("", "configuration", "create", "foo", "--from-file", "MISSING")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("filesystem error: open MISSING: no such file or directory"))
		})

		Describe("directory", func() {
			var tmpDir string
			var mkdirErr error

			BeforeEach(func() {
				tmpDir, mkdirErr = os.MkdirTemp("", "epinio-config")
				Expect(mkdirErr).ToNot(HaveOccurred())

				DeferCleanup(func() {
					err := os.RemoveAll(tmpDir)
					Expect(err).ToNot(HaveOccurred())
				})
			})

			It("fails for a directory", func() {
				out, err := env.Epinio("", "configuration", "create", "foo", "--from-file", tmpDir)
				Expect(err).To(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("filesystem error: read %s: is a directory", tmpDir))
			})
		})
	})

	Describe("configuration delete", func() {

		BeforeEach(func() {
			env.MakeConfiguration(configurationName1)
		})

		It("deletes a configuration", func() {
			env.DeleteConfigurations(configurationName1)
		})

		It("deletes multiple configurations", func() {
			env.MakeConfiguration(configurationName2)
			env.DeleteConfigurations(configurationName1, configurationName2)
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
			Expect(out).To(ContainSubstring("Configurations Removed"))

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

		Context("command completion", func() {

			BeforeEach(func() {
				env.MakeConfiguration(configurationName2)
			})

			AfterEach(func() {
				env.CleanupConfiguration(configurationName1)
				env.CleanupConfiguration(configurationName2)
			})

			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "delete", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(configurationName1))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "delete", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does match for more than one configuration", func() {
				// Completion behavior can vary slightly between shells/versions, so keep this
				// as a smoke test that both configuration names are offered at the right time
				// instead of asserting on exact filtering semantics.
				out, err := env.Epinio("", "__complete", "configuration", "delete", configurationName1, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(configurationName2))

				out, err = env.Epinio("", "__complete", "configuration", "delete", configurationName2, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(configurationName1))
			})
		})
	})

	Describe("configuration bind", func() {
		var appName string

		BeforeEach(func() {
			appName = catalog.NewAppName()

			env.MakeConfiguration(configurationName1)
			env.MakeContainerImageApp(appName, 1, containerImageURL)
		})

		It("binds a configuration to the application deployment", func() {
			env.BindAppConfiguration(appName, configurationName1, namespace)
		})

		Context("command completion", func() {
			It("matches empty configuration prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "bind", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(configurationName1))
			})

			It("matches empty app prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "bind", configurationName1, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(appName))
			})

			It("does not match unknown configuration prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "bind", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match unknown app prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "bind", configurationName1, "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "configuration", "bind", configurationName1, appName, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(appName))
				Expect(out).ToNot(ContainSubstring(configurationName1))
			})
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

		It("unbinds a configuration from the application deployment", func() {
			env.UnbindAppConfiguration(appName, configurationName1, namespace)
		})

		Context("command completion", func() {
			It("matches empty configuration prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "unbind", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(configurationName1))
			})

			It("matches empty app prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "unbind", configurationName1, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(appName))
			})

			It("does not match unknown configuration prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "unbind", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match unknown app prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "unbind", configurationName1, "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "configuration", "unbind", configurationName1, appName, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(appName))
				Expect(out).ToNot(ContainSubstring(configurationName1))
			})
		})
	})

	Describe("configuration show", func() {

		It("shows configuration details", func() {
			env.MakeConfiguration(configurationName1)

			out, err := env.Epinio("", "configuration", "show", configurationName1)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Configuration Details"))
			// Be robust to masking and formatting changes: just ensure the table headers,
			// parameter name and access path are present.
			Expect(out).To(ContainSubstring("PARAMETER"))
			Expect(out).To(ContainSubstring("ACCESS PATH"))
			Expect(out).To(ContainSubstring("username"))
			Expect(out).To(ContainSubstring("/configurations/" + configurationName1 + "/username"))
		})

		It("shows a configuration in JSON format", func() {
			env.MakeConfiguration(configurationName1)

			out, err := env.Epinio("", "configuration", "show", configurationName1, "--output", "json")
			Expect(err).ToNot(HaveOccurred(), out)

			configuration := models.ConfigurationResponse{}
			err = json.Unmarshal([]byte(extractJSONPayload(out)), &configuration)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(configuration.Meta.Name).To(Equal(configurationName1))
		})

		It("reads from files, and truncates large configuration details", func() {
			env.MakeConfigurationFromFiles(configurationName1, testenv.TestAssetPath("config.yaml"))

			out, err := env.Epinio("", "configuration", "show", configurationName1)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Configuration Details"))
			// When values are masked (****), truncation text like "hiding N bytes" may not appear.
			// Just verify the file parameter and its access path are shown.
			Expect(out).To(ContainSubstring("file"))
			Expect(out).To(ContainSubstring("/configurations/" + configurationName1 + "/file"))
		})

		Context("command completion", func() {
			BeforeEach(func() {
				env.MakeConfiguration(configurationName1)
			})

			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "show", "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(configurationName1))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "configuration", "show", "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "configuration", "show", configurationName1, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(configurationName1))
			})
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

		It("it edits the configuration, and restarts the app", func() {
			// edit the configuration ...

			out, err := env.Epinio("", "configuration", "update", configurationName1,
				"--unset", "username",
				"--set", "user=ci/cd",
			)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("Update Configuration"))
			// Check that the diff is rendered without asserting on the exact value formatting.
			Expect(out).To(ContainSubstring("PARAMETER"))
			Expect(out).To(ContainSubstring("OP"))
			Expect(out).To(ContainSubstring("username"))
			Expect(out).To(ContainSubstring("remove"))
			Expect(out).To(ContainSubstring("user"))
			Expect(out).To(ContainSubstring("add/change"))
			Expect(out).To(ContainSubstring("Configuration Changes Saved"))

			// Confirm the changes ...

			out, err = env.Epinio("", "configuration", "show", configurationName1)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("Configuration Details"))
			// Values are masked; just ensure the parameter and access path are present.
			Expect(out).To(ContainSubstring("user"))
			Expect(out).To(ContainSubstring("/configurations/" + configurationName1 + "/user"))

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

		Context("with --no-restart flag", func() {
			getPodNames := func(namespace, app string) ([]string, error) {
				podName, err := proc.Kubectl(
					"get",
					"pods",
					"-n",
					namespace,
					"-l",
					fmt.Sprintf("app.kubernetes.io/name=%s", app),
					"-o",
					"jsonpath='{.items[*].metadata.name}'",
				)
				return strings.Split(strings.Trim(podName, "'"), " "), err
			}

			It("updates configuration without restarting bound apps", func() {
				By("getting pod names before update")
				oldPodNames, err := getPodNames(namespace, appName)
				Expect(err).ToNot(HaveOccurred())

				By("updating configuration with --no-restart")
				out, err := env.Epinio("", "configuration", "update", configurationName1,
					"--no-restart",
					"--set", "newkey=newvalue",
				)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Configuration Changes Saved"))

				By("verifying changes were applied")
				out, err = env.Epinio("", "configuration", "show", configurationName1)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Configuration Details"))
				Expect(out).To(ContainSubstring("newkey"))

				By("verifying pods DID NOT restart")
				Consistently(func() []string {
					names, err := getPodNames(namespace, appName)
					Expect(err).ToNot(HaveOccurred())
					return names
				}, "15s", "2s").Should(ContainElements(oldPodNames))

				By("verifying app is still healthy")
				out, err = env.Epinio("", "app", "show", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Status", "1/1"),
					),
				)
			})

			It("updates configuration and restarts bound apps by default (without --no-restart)", func() {
				By("getting pod names before update")
				oldPodNames, err := getPodNames(namespace, appName)
				Expect(err).ToNot(HaveOccurred())

				By("updating configuration WITHOUT --no-restart (default behavior)")
				out, err := env.Epinio("", "configuration", "update", configurationName1,
					"--set", "anotherkey=anothervalue",
				)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Configuration Changes Saved"))

				By("verifying pods DID restart (default behavior)")
				Eventually(func() []string {
					names, err := getPodNames(namespace, appName)
					Expect(err).ToNot(HaveOccurred())
					return names
				}, "2m", "2s").ShouldNot(ContainElements(oldPodNames))

				By("verifying app is healthy after restart")
				Eventually(func() string {
					out, err := env.Epinio("", "app", "list")
					Expect(err).ToNot(HaveOccurred(), out)
					return out
				}, "2m").Should(
					HaveATable(
						WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
						WithRow(appName, WithDate(), "1/1", appName+".*", configurationName1, ""),
					),
				)
			})
		})
	})

	Context("service-owned configurations", func() {
		//var catalogService models.CatalogService
		var service, appName, chart, config string

		BeforeEach(func() {
			service = catalog.NewServiceName()

			By("make service instance: " + service)
			out, err := env.Epinio("", "service", "create", "postgresql-dev", service)
			Expect(err).ToNot(HaveOccurred(), out)

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "show", service)
				return out
			}, ServiceDeployTimeout, ServiceDeployPollingInterval).Should(
				HaveATable(WithRow("Status", "deployed")),
			)

			appName = catalog.NewAppName()
			By("make app: " + appName)
			env.MakeContainerImageApp(appName, 1, containerImageURL)

			chart = names.ServiceReleaseName(service)
			config = chart + "-postgresql"

			By("chart: " + chart)
			By("config: " + config)

			// NOTE: The bind/unbind cycle below materializes the configuration of the service

			By("bind service: " + service)

			out, err = env.Epinio("", "service", "bind", service, appName)
			Expect(err).ToNot(HaveOccurred(), out)

			By("wait for bound")
			Eventually(func() string {
				out, _ := env.Epinio("", "app", "show", appName)
				return out
			}, "2m", "5s").Should(HaveATable(WithRow("Bound Configurations", config)))

			By("done before")
		})

		It("doesn't unbind a service-owned configuration", func() {
			out, err := env.Epinio("", "configuration", "unbind", config, appName)
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Configuration '%s' belongs to service", config))
		})

		It("doesn't delete a bound service-owned configuration", func() {
			out, err := env.Epinio("", "configuration", "delete", config)
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Configuration '%s' belongs to service", config))
		})

		It("doesn't delete any service-owned configuration", func() {
			By("unbind service: " + appName)

			out, err := env.Epinio("", "service", "unbind", service, appName)
			Expect(err).ToNot(HaveOccurred(), out)

			By("wait for unbound")
			Eventually(func() string {
				out, _ := env.Epinio("", "app", "show", appName)
				return out
			}, "2m", "5s").ShouldNot(HaveATable(WithRow("Bound Configurations", config)))

			out, err = env.Epinio("", "configuration", "delete", config)
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Configuration '%s' belongs to service", config))
		})

		It("deletes a service-owned configuration after service deletion", func() {
			By("unbind service: " + appName)

			// Unbind can be racy with binding propagation; allow a short window to succeed.
			var out string
			var err error
			Eventually(func() bool {
				out, err = env.Epinio("", "service", "unbind", service, appName)
				return err == nil
			}, "2m", "10s").Should(BeTrue(), "service unbind should succeed; last out: %s", out)

			By("wait for unbound")
			Eventually(func() string {
				out, _ := env.Epinio("", "app", "show", appName)
				return out
			}, "2m", "5s").ShouldNot(HaveATable(WithRow("Bound Configurations", config)))

			out, err = env.Epinio("", "configuration", "delete", config)
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Configuration '%s' belongs to service", config))

			By("remove app: " + appName)
			env.DeleteApp(appName)

			// The preceding removed the service/config binding as well, allowing us to
			// remove the service and its configs without care.

			By("remove service instance: " + service)

			out, err = env.Epinio("", "service", "delete", service)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Services Removed"))

			Eventually(func() string {
				out, _ := env.Epinio("", "service", "delete", service)
				return out
			}, "1m", "5s").Should(ContainSubstring("service '%s' does not exist", service))

			By("done after")
			// No cleanup of configurationName1 here: this context only creates service-owned config `config`,
			// which is removed with the service.
		})
	})

})
