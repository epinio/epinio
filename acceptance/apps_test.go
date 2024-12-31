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
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/internal/api/v1/application"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/internal/routes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps", LApplication, func() {
	var (
		namespace string
		appName   string
	)
	containerImageURL := "epinio/sample-app"
	wordpress := "https://github.com/epinio/example-wordpress"
	privateRepo := "https://github.com/epinio/example-go-private"
	wpBuilder := "paketobuildpacks/builder:0.2.443-full"

	// defaultBuilder := "paketobuildpacks/builder:full"
	defaultBuilder := "paketobuildpacks/builder-jammy-full:0.3.290"
	// tinyBuilder := "paketobuildpacks/builder:tiny"
	tinyBuilder := "paketobuildpacks/builder-jammy-tiny:0.0.197"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		appName = catalog.NewAppName()
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Describe("application create failures", func() {
		It("rejects names not fitting kubernetes requirements", func() {
			out, err := env.Epinio("", "app", "create", "BOGUS")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("name must consist of lower case alphanumeric"))
		})
	})
	When("creating an application without a workload", func() {
		AfterEach(func() {
			// MakeApp... by each test (It)
			env.DeleteApp(appName)
		})

		It("creates the app", func() {
			out, err := env.Epinio("", "app", "create", appName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Ok"))
		})

		Context("with configuration", func() {
			var configurationName string

			BeforeEach(func() {
				configurationName = catalog.NewConfigurationName()
				env.MakeConfiguration(configurationName)
			})

			AfterEach(func() {
				env.DeleteConfigurationsUnbind(configurationName)
				// env.DeleteApp see outer context
			})

			It("creates the app with instance count, configurations, and environment", func() {
				out, err := env.Epinio("", "app", "create", appName,
					"--app-chart", "standard",
					"--bind", configurationName,
					"--instances", "2",
					"--env", "CREDO=up",
					"--env", "DOGMA=no",
					"--env", "COMPLEX=-X foo=bar",
					"--env", "COMPLEXB=-Xbab -Xaba",
				)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Ok"))

				out, err = env.Epinio("", "app", "show", appName)
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(out).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Origin", "<<undefined>>"),
						WithRow("Created", WithDate()),
						WithRow("Status", "not deployed"),
						WithRow("Desired Routes", ""),
						WithRow("", appName+".*"),
						WithRow("App Chart", "standard"),
						WithRow("Desired Instances", "2"),
						WithRow("Bound Configurations", configurationName),
						WithRow("Environment", ""),
						WithRow("- COMPLEX", "-X foo=bar"),
						WithRow("- COMPLEXB", "-Xbab -Xaba"),
						WithRow("- CREDO", "up"),
						WithRow("- DOGMA", "no"),
					),
				)
			})

			Context("manifest", func() {
				destinationPath := catalog.NewTmpName("tmpManifest") + `.yaml`

				AfterEach(func() {
					// Remove transient manifest
					out, err := proc.Run("", false, "rm", "-f", destinationPath)
					Expect(err).ToNot(HaveOccurred(), out)
				})

				It("is possible to get a manifest", func() {
					out, err := env.Epinio("", "app", "create", appName,
						"--app-chart", "standard",
						"--bind", configurationName,
						"--instances", "2",
						"--env", "CREDO=up",
						"--env", "DOGMA=no")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).To(ContainSubstring("Ok"))

					out, err = env.Epinio("", "app", "manifest", appName, destinationPath)
					Expect(err).ToNot(HaveOccurred(), out)

					manifest, err := os.ReadFile(destinationPath)
					Expect(err).ToNot(HaveOccurred(), destinationPath)

					theManifest := models.ApplicationManifest{}
					err = yaml.Unmarshal(manifest, &theManifest)
					Expect(err).ToNot(HaveOccurred(), string(manifest))

					// Note: Cannot use MatchYaml because of the `HavePrefix` check on the route.
					// The MatchYAML asserts equality and we do not have the full route name here to put in.
					Expect(theManifest.Name).To(Equal(appName))
					var i int32 = 2
					Expect(theManifest.Configuration.Instances).To(Equal(&i))
					Expect(theManifest.Configuration.Configurations).To(HaveLen(1))
					Expect(theManifest.Configuration.Routes).To(HaveLen(1))
					Expect(theManifest.Configuration.Configurations[0]).To(Equal(configurationName))
					Expect(theManifest.Configuration.Routes[0]).To(HavePrefix(appName))
					Expect(theManifest.Configuration.Environment).To(HaveLen(2))
					Expect(theManifest.Configuration.Environment).To(HaveKeyWithValue("CREDO", "up"))
					Expect(theManifest.Configuration.Environment).To(HaveKeyWithValue("DOGMA", "no"))
					Expect(theManifest.Configuration.AppChart).To(Equal("standard"))
					Expect(theManifest.Namespace).To(Equal(namespace))
				})
			})
		})

		It("creates the app with environment variables", func() {
			out, err := env.Epinio("", "app", "create", appName, "--env", "MYVAR=myvalue")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Ok"))

			out, err = env.Epinio("", "apps", "env", "list", appName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring(`MYVAR`))
			Expect(out).To(ContainSubstring(`myvalue`))
		})

		When("pushing a workload", func() {
			BeforeEach(func() {
				out, err := env.Epinio("", "app", "create", appName)
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("creates the workload", func() {
				appDir := "../assets/sample-app"
				out, err := env.EpinioPush(appDir, appName, "--name", appName,
					"--env", "CREDO=up",
					"--env", "DOGMA=no",
					"--env", "COMPLEX=-X foo=bar",
					"--env", "COMPLEXB=-Xbab -Xaba",
				)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("App is online"))
			})
		})
	})

	When("pushing an app", func() {
		It("rejects mixed origins", func() {
			out, err := env.Epinio("", "push",
				"--name", appName,
				"--git", wordpress,
				"--container-image-url", containerImageURL)
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Cannot use `--path`, `--git`, and `--container-image-url` options together"))
		})
	})

	When("pushing an app from an external repository", func() {
		It("rejects a bad provider specification", func() {
			out, err := env.Epinio("", "push",
				"--name", appName,
				"--git", wordpress,
				"--git-provider", "bogus")
			Expect(err).To(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("Bad --git-provider `bogus`"))
		})

		It("rejects a bad provider specification for a wrong git url", func() {
			out, err := env.Epinio("", "push",
				"--name", appName,
				"--git", wordpress,
				"--git-provider", "gitlab")
			Expect(err).To(HaveOccurred(), out)

			Expect(out).To(ContainSubstring("git url and provider mismatch"))
		})

		It("rejects a bad specification", func() {
			out, err := env.Epinio("", "push",
				"--name", appName,
				"--git", wordpress+",main,borken")
			Expect(err).To(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Bad --git reference git `" +
				wordpress +
				",main,borken`, expected `repo?,rev?` as value"))
		})

		It("pushes the app successfully (repository alone)", func() {
			pushLog, err := env.EpinioPush("",
				appName,
				"--name", appName,
				"--git", wordpress,
				"--builder-image", wpBuilder,
				"-e", "BP_PHP_WEB_DIR=wordpress",
				"-e", "BP_PHP_VERSION=8.0.x",
				"-e", "BP_PHP_SERVER=nginx")
			Expect(err).ToNot(HaveOccurred(), pushLog)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(
				HaveATable(
					WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(appName, WithDate(), "1/1", appName+".*", "", ""),
				),
			)

			By("deleting the app")
			env.DeleteApp(appName)
		})

		It("pushes the app successfully (repository + branch name)", func() {
			pushLog, err := env.EpinioPush("",
				appName,
				"--name", appName,
				"--git", wordpress+",main",
				"--builder-image", wpBuilder,
				"-e", "BP_PHP_WEB_DIR=wordpress",
				"-e", "BP_PHP_VERSION=8.0.x",
				"-e", "BP_PHP_SERVER=nginx")
			Expect(err).ToNot(HaveOccurred(), pushLog)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(
				HaveATable(
					WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(appName, WithDate(), "1/1", appName+".*", "", ""),
				),
			)

			By("deleting the app")
			env.DeleteApp(appName)
		})

		It("pushes the app successfully (repository + commit id)", func() {
			pushLog, err := env.EpinioPush("",
				appName,
				"--name", appName,
				"--git", wordpress+",68af5bad11d8f3b95bdf547986fe3348324919c5",
				"--builder-image", wpBuilder,
				"-e", "BP_PHP_WEB_DIR=wordpress",
				"-e", "BP_PHP_VERSION=8.0.x",
				"-e", "BP_PHP_SERVER=nginx")
			Expect(err).ToNot(HaveOccurred(), pushLog)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(
				HaveATable(
					WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(appName, WithDate(), "1/1", appName+".*", "", ""),
				),
			)

			By("deleting the app")
			env.DeleteApp(appName)
		})

		When("pushing an app from a private repository", func() {

			It("rejects without a token", func() {
				out, err := env.Epinio("", "push", "--name", appName, "--git", privateRepo)
				Expect(err).To(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("authentication required"))
			})

			It("pushes the app when providing a proper token", func() {
				env.MakeGitconfig(catalog.NewGitconfigName())

				out, err := env.Epinio("", "push", "--name", appName, "--git", privateRepo)
				Expect(err).ToNot(HaveOccurred(), out)
			})
		})

		Describe("update", func() {
			BeforeEach(func() {
				pushLog, err := env.EpinioPush("",
					appName,
					"--name", appName,
					"--git", wordpress+",main",
					"--builder-image", wpBuilder,
					"-e", "BP_PHP_WEB_DIR=wordpress",
					"-e", "BP_PHP_VERSION=8.0.x",
					"-e", "BP_PHP_SERVER=nginx")
				Expect(err).ToNot(HaveOccurred(), pushLog)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "list")
					Expect(err).ToNot(HaveOccurred(), out)
					return out
				}, "5m").Should(
					HaveATable(
						WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
						WithRow(appName, WithDate(), "1/1", appName+".*", "", ""),
					),
				)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
					return out
				}, "1m").Should(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Status", "1/1"),
					),
				)
			})

			It("respects the desired number of instances", func() {
				out, err := env.Epinio("", "app", "update", appName, "-i", "3")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
					return out
				}, "1m").Should(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Status", "3/3"),
					),
				)
			})

			It("respects route changes", func() {
				route := "mycustomdomain.org/api"

				out, err := env.Epinio("", "app", "update", appName, "-r", route)
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
					return out
				}, "1m").Should(HaveATable(WithRow(".*", route)))
			})

			It("respects complete route removal", func() {
				out, err := env.Epinio("", "app", "update", appName, "--clear-routes")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "list")
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
					return out
				}, "1m").Should(HaveATable(
					WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(appName, WithDate(), "1/1", "<<none>>", "", ""),
				))
			})

			It("ignores scheme prefixes in routes", func() {
				route := "mycustomdomain.org/api"

				out, err := env.Epinio("", "app", "update", appName,
					"-r", "https://"+route)
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
					return out
				}, "1m").Should(HaveATable(WithRow(".*", route)))
			})

			Context("app charts", func() {
				var chartName string
				var tempFile string

				BeforeEach(func() {
					chartName = catalog.NewTmpName("chart-")
					tempFile = env.MakeAppchart(chartName)
				})

				AfterEach(func() {
					env.DeleteAppchart(tempFile)
				})

				It("fails to change the app chart of the running app", func() {
					out, err := env.Epinio("", "app", "update", appName,
						"--app-chart", chartName)
					Expect(err).To(HaveOccurred(), out)
					Expect(out).To(ContainSubstring("unable to change app chart of active application"))
				})

				When("no workload is present", func() {
					var appName1 string

					BeforeEach(func() {
						appName1 = catalog.NewAppName()

						out, err := env.Epinio("", "app", "create", appName1, "--app-chart", chartName)
						Expect(err).ToNot(HaveOccurred(), out)
						Expect(out).To(ContainSubstring("Ok"))

						out, err = env.Epinio("", "app", "show", appName1)
						Expect(err).ToNot(HaveOccurred(), out)

						Expect(out).To(
							HaveATable(
								WithHeaders("KEY", "VALUE"),
								WithRow("App Chart", chartName),
							),
						)
					})

					AfterEach(func() {
						env.DeleteApp(appName1)
					})

					It("respects the desired app chart", func() {
						out, err := env.Epinio("", "app", "update", appName1,
							"--app-chart", "standard")
						Expect(err).ToNot(HaveOccurred(), out)

						Eventually(func() string {
							out, err := env.Epinio("", "app", "show", appName1)
							ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

							return out
						}, "1m").Should(
							HaveATable(
								WithHeaders("KEY", "VALUE"),
								WithRow("App Chart", "standard"),
							),
						)
					})
				})
			})

			It("respects environment variable changes", func() {
				out, err := env.Epinio("", "app", "update", appName, "--env", "MYVAR=myvalue")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					out, err := env.Epinio("", "apps", "env", "list", appName)
					Expect(err).ToNot(HaveOccurred(), out)
					return out
				}, "2m").Should(
					HaveATable(
						WithHeaders("VARIABLE", "VALUE"),
						WithRow("MYVAR", "myvalue"),
					),
				)
			})

			AfterEach(func() {
				env.DeleteApp(appName)
			})
		})
	})

	Describe("restage", func() {
		When("restaging an existing and running app", func() {
			BeforeEach(func() {
				env.MakeApp(appName, 1, false)
			})
			AfterEach(func() {
				env.DeleteApp(appName)
			})
			It("will be staged again, and restarted", func() {
				restageLogs, err := env.Epinio("", "app", "restage", appName)
				Expect(err).ToNot(HaveOccurred(), restageLogs)
				Expect(restageLogs).To(ContainSubstring("Restaging and restarting application"))
				Expect(restageLogs).To(ContainSubstring("Restarting application"))
			})
		})

		When("restaging an existing and inactive app", func() {
			BeforeEach(func() {
				env.MakeApp(appName, 0, false)
			})
			AfterEach(func() {
				env.DeleteApp(appName)
			})
			It("will be staged again, and NOT restarted", func() {
				restageLogs, err := env.Epinio("", "app", "restage", appName)
				Expect(err).ToNot(HaveOccurred(), restageLogs)
				Expect(restageLogs).To(ContainSubstring("Restaging application"))
				Expect(restageLogs).ToNot(ContainSubstring("Restarting application"))
			})
		})

		When("restaging an existing and running app, with restart suppressed", func() {
			BeforeEach(func() {
				env.MakeApp(appName, 1, false)
			})
			AfterEach(func() {
				env.DeleteApp(appName)
			})
			It("will be staged again, and NOT restarted", func() {
				restageLogs, err := env.Epinio("", "app", "restage", "--no-restart", appName)
				Expect(err).ToNot(HaveOccurred(), restageLogs)
				Expect(restageLogs).To(ContainSubstring("Restaging application"))
				Expect(restageLogs).ToNot(ContainSubstring("Restarting application"))
			})
		})

		When("restaging a non existing app", func() {
			It("will return an error", func() {
				restageLogs, err := env.Epinio("", "app", "restage", appName)
				Expect(err).To(HaveOccurred(), restageLogs)
			})
		})

		When("restaging a container based app", func() {
			It("won't be staged", func() {
				env.MakeContainerImageApp(appName, 1, containerImageURL)

				restageLogs, err := env.Epinio("", "app", "restage", appName)
				Expect(err).ToNot(HaveOccurred(), restageLogs)
				Expect(restageLogs).Should(ContainSubstring("Unable to restage container-based application"))

				By("deleting the app")
				env.DeleteApp(appName)
			})
		})

	})

	When("pushing as a stateful app", func() {
		var chartName string
		var tempFile string

		BeforeEach(func() {
			chartName = catalog.NewTmpName("chart-")
			tempFile = env.MakeAppchartStateful(chartName)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
			env.DeleteAppchart(tempFile)
		})

		It("pushes successfully", func() {
			pushLog, err := env.EpinioPush("../assets/sample-app",
				appName,
				"--app-chart", chartName,
				"--name", appName)
			Expect(err).ToNot(HaveOccurred(), pushLog)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(
				HaveATable(
					WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(appName, WithDate(), "1/1", appName+".*", "", ""),
				),
			)

			out, err := proc.Kubectl("get", "statefulset",
				"--namespace", namespace,
				"--selector=app.kubernetes.io/name="+appName,
				"-o", `jsonpath={.items[*].spec.template.metadata.labels.app\.kubernetes\.io/name}`)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal(appName))
		})
	})

	When("pushing with --clear-routes flag (= no routes)", func() {
		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("creates no ingresses", func() {
			pushOutput, err := env.Epinio("", "apps", "push",
				"--name", appName,
				"--container-image-url", containerImageURL,
				"--clear-routes",
			)
			Expect(err).ToNot(HaveOccurred(), pushOutput)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").Should(HaveATable(
				WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
				WithRow(appName, WithDate(), "1/1", "<<none>>", "", ""),
			))

			Consistently(func() string {
				out, err := proc.Kubectl("get", "ingress",
					"--namespace", namespace,
					"--selector=app.kubernetes.io/name="+appName,
				)
				Expect(err).NotTo(HaveOccurred(), out)
				return out
			}, 1*time.Minute, 5*time.Second).Should(ContainSubstring("No resources found"))
		})
	})

	When("pushing with custom route flag", func() {
		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("creates an ingress matching the custom route", func() {
			route := "mycustomdomain.org/api"
			pushOutput, err := env.Epinio("", "apps", "push",
				"--name", appName,
				"--container-image-url", containerImageURL,
				"--route", route,
			)
			Expect(err).ToNot(HaveOccurred(), pushOutput)

			routeObj := routes.FromString(route)
			out, err := proc.Kubectl("get", "ingress",
				"--namespace", namespace,
				"--selector=app.kubernetes.io/name="+appName,
				"-o", "jsonpath={.items[*].spec.rules[0].host}")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal(routeObj.Domain))

			out, err = proc.Kubectl("get", "ingress",
				"--namespace", namespace,
				"--selector=app.kubernetes.io/name="+appName,
				"-o", "jsonpath={.items[*].spec.rules[0].http.paths[0].path}")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal(routeObj.Path))

			out, err = proc.Kubectl("get", "app",
				"--namespace", namespace, appName,
				"-o", "jsonpath={.spec.routes[0]}")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal(route))
		})

		It("ignores scheme prefixes in the custom route", func() {
			route := "mycustomdomain.org/api"
			pushOutput, err := env.Epinio("", "apps", "push",
				"--name", appName,
				"--container-image-url", containerImageURL,
				"--route", "http://"+route,
			)
			Expect(err).ToNot(HaveOccurred(), pushOutput)

			routeObj := routes.FromString(route)
			out, err := proc.Kubectl("get", "ingress",
				"--namespace", namespace,
				"--selector=app.kubernetes.io/name="+appName,
				"-o", "jsonpath={.items[*].spec.rules[0].host}")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal(routeObj.Domain))

			out, err = proc.Kubectl("get", "ingress",
				"--namespace", namespace,
				"--selector=app.kubernetes.io/name="+appName,
				"-o", "jsonpath={.items[*].spec.rules[0].http.paths[0].path}")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal(routeObj.Path))

			out, err = proc.Kubectl("get", "app",
				"--namespace", namespace, appName,
				"-o", "jsonpath={.spec.routes[0]}")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal(route))
		})
	})

	When("pushing with custom builder flag", func() {
		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("uses the custom builder to stage", func() {
			By("Pushing a golang app")
			appDir := "../assets/golang-sample-app"
			pushLog, err := env.EpinioPush(appDir,
				appName,
				"--name", appName,
				"--builder-image", tinyBuilder)
			Expect(err).ToNot(HaveOccurred(), pushLog)

			By("checking if the staging is using custom builder image")
			labels := fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/component=staging", appName)
			imageList, err := proc.Kubectl("get", "pod",
				"--namespace", testenv.Namespace,
				"-l", labels,
				"-o", "jsonpath={.items[0].spec.containers[*].image}")
			Expect(err).NotTo(HaveOccurred())
			Expect(imageList).To(ContainSubstring(tinyBuilder))
		})
	})

	When("pushing a failed application", func() {
		// NOTE: The staging of the application is OK.
		// It is the actual deployment that fails!

		var tmpDir string
		var err error
		BeforeEach(func() {
			By("Pushing an app that will fail")
			tmpDir, err = os.MkdirTemp("", "epinio-failing-app")
			Expect(err).ToNot(HaveOccurred())
			appCode := []byte("\n<?php\nphpinfo();\n?>\n")
			err = os.WriteFile(path.Join(tmpDir, "index.php"), appCode, 0644)
			Expect(err).ToNot(HaveOccurred())
			badProcfile := []byte("web: doesntexist")
			err = os.WriteFile(path.Join(tmpDir, "Procfile"), badProcfile, 0644)
			Expect(err).ToNot(HaveOccurred())

			// Don't block because this push will only exit if it times out
			go func() {
				defer GinkgoRecover()
				// Ignore any errors. When the main thread pushes the app again,
				// this command will probably fail with an error because the helm release
				// will be deleted by the other `push`.
				_, _ = env.EpinioPush(tmpDir, appName, "--name", appName,
					"--builder-image", defaultBuilder)
			}()

			// Wait until previous staging job is complete
			By("waiting for the old staging job to complete")
			Eventually(func() error {
				statusJSON, err := proc.Kubectl("get", "jobs", "-A",
					"-l", fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s", appName, namespace),
					"-o", "jsonpath={.items[].status['conditions'][]}")
				if err != nil {
					return err
				}

				var status map[string]string
				err = json.Unmarshal([]byte(statusJSON), &status)
				if err != nil {
					return err
				}

				if status["type"] != "Complete" || status["status"] != "True" {
					return errors.New("staging job not complete")
				}

				return nil
			}, 3*time.Minute, 3*time.Second).ShouldNot(HaveOccurred())

		})

		AfterEach(func() {
			env.DeleteApp(appName)
			os.RemoveAll(tmpDir)
		})

		It("shows the proper status", func() {
			out, err := env.Epinio("", "app", "show", appName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Status", "((0/1)|(staging ok, deployment failed))"),
				),
			)

			out, err = env.Epinio("", "app", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(
				HaveATable(
					WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(appName, WithDate(), "0/1", ".*", "", ".*"),
				),
			)
		})

		It("succeeds when re-pushing a fix", func() {
			// Fix the problem (so that the app now deploys fine) and push again
			By("fixing the problem and pushing the application again")
			os.Remove(path.Join(tmpDir, "Procfile"))
			out, err := env.EpinioPush(tmpDir, appName, "--name", appName,
				"--builder-image", defaultBuilder)
			Expect(err).ToNot(HaveOccurred(), out)
		})
	})

	When("pushing a failed staging application", func() {

		var tmpDir string
		var err error

		BeforeEach(func() {
			By("Pushing an app that will fail")
			tmpDir, err = os.MkdirTemp("", "epinio-failing-app")
			Expect(err).ToNot(HaveOccurred())

			DeferCleanup(func() {
				os.RemoveAll(tmpDir)
			})

			err = os.WriteFile(path.Join(tmpDir, "empty"), []byte(""), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Don't block because this push will only exit if it times out
			go func() {
				defer GinkgoRecover()
				// Ignore any errors. When the main thread pushes the app again,
				// this command will probably fail with an error because the helm release
				// will be deleted by the other `push`.
				_, _ = env.EpinioPush(tmpDir, appName, "--name", appName)
			}()

			// Wait until previous staging job is complete
			By("waiting for the old staging job to fail")
			Eventually(func() string {
				statusJSON, err := proc.Kubectl(
					"get", "jobs", "-A",
					"-l", fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s", appName, namespace),
					"-o", "jsonpath={.items[].status['conditions'][]}",
				)
				if err != nil || statusJSON == "" {
					return ""
				}

				var status map[string]string
				err = json.Unmarshal([]byte(statusJSON), &status)
				Expect(err).ToNot(HaveOccurred())

				return status["type"]
			}, 3*time.Minute, 3*time.Second).Should(BeEquivalentTo("Failed"))
		})

		It("succeeds when re-pushing a fix", func() {
			// Fix the problem (so that the app now deploys fine) and push again
			By("fixing the problem and pushing the application again")

			os.Remove(path.Join(tmpDir, "empty"))
			appCode := []byte("\n<?php\nphpinfo();\n?>\n")

			err := os.WriteFile(path.Join(tmpDir, "index.php"), appCode, 0644)
			Expect(err).ToNot(HaveOccurred())

			out, err := env.EpinioPush(tmpDir, appName, "--name", appName, "--instances", "2")
			Expect(err).ToNot(HaveOccurred(), out)
		})
	})

	When("pushing an app multiple times", func() {
		var (
			timeout  = 30 * time.Second
			interval = 1 * time.Second
		)

		act := func(name string, arg ...string) (string, error) {
			appDir := "../assets/sample-app"
			out, err := env.EpinioPush(appDir, name, append([]string{"--name", name}, arg...)...)
			return out, err
		}

		replicas := func(ns, name string) string {
			n, err := proc.Kubectl("get", "deployments",
				"-l", fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/part-of=%s", name, ns),
				"--namespace", ns,
				"-o", "jsonpath={.items[].spec.replicas}")
			if err != nil {
				return ""
			}
			return n
		}

		It("pushes the same app again successfully", func() {
			env.MakeApp(appName, 1, false)

			By("pushing the app again")
			env.MakeApp(appName, 1, false)

			By("deleting the app")
			env.DeleteApp(appName)
		})

		It("honours the given instance count", func() {
			By("pushing without instance count", func() {
				out, err := act(appName)
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					return replicas(namespace, appName)
				}, timeout, interval).Should(Equal(strconv.Itoa(int(application.DefaultInstances))))
			})
			By("pushing with 0 instance count", func() {
				out, err := act(appName, "--instances", "0")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					return replicas(namespace, appName)
				}, timeout, interval).Should(Equal("0"))
			})
			By("pushing with an instance count", func() {
				out, err := act(appName, "--instances", "2")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					return replicas(namespace, appName)
				}, timeout, interval).Should(Equal("2"))
			})
			By("pushing again, without an instance count", func() {
				out, err := act(appName)
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					return replicas(namespace, appName)
				}, timeout, interval).Should(Equal("2"))
			})
		})
	})

	Describe("build cache", func() {
		push := func(arg ...string) (string, error) {
			appDir := "../assets/sample-app"
			out, err := env.EpinioPush(appDir, appName, append([]string{"--name", appName}, arg...)...)
			return out, err
		}
		BeforeEach(func() {
			out, err := push()
			Expect(err).ToNot(HaveOccurred(), out)
		})

		When("pushing for the second time", func() {
			AfterEach(func() {
				env.DeleteApp(appName)
			})

			It("is using the cache PVC", func() {
				out, err := proc.Kubectl("get", "pvc", "--namespace",
					testenv.Namespace, names.GenerateResourceName(namespace, appName))
				Expect(err).ToNot(HaveOccurred(), out)

				out, err = push()
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(out).To(ContainSubstring("Reusing cached layer"))
			})
		})
		When("deleting the app", func() {
			It("deletes the cache PVC too", func() {
				out, err := proc.Kubectl("get", "pvc", "--namespace",
					testenv.Namespace, names.GenerateResourceName(namespace, appName))
				Expect(err).ToNot(HaveOccurred(), out)
				env.DeleteApp(appName)

				out, err = proc.Kubectl("get", "pvc", "--namespace",
					testenv.Namespace, names.GenerateResourceName(namespace, appName))
				Expect(err).To(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(`persistentvolumeclaims "%s" not found`, names.GenerateResourceName(namespace, appName)))
			})
		})
	})

	Describe("push and delete", func() {
		It("shows the staging logs", func() {
			By("pushing the app")
			out := env.MakeApp(appName, 1, true)

			Expect(out).To(ContainSubstring(`Generating default PHP configuration`))
			// Doesn't include linkerd sidecar logs
			Expect(out).ToNot(ContainSubstring(`linkerd-`))
		})

		It("deploys a golang app", func() {
			out := env.MakeGolangApp(appName, 1, true)

			By("checking for the application resource", func() {
				Eventually(func() string {
					out, _ := proc.Kubectl("get", "app",
						"--namespace", namespace, appName)
					return out
				}, "1m").Should(ContainSubstring("AGE")) // this checks for the table header from kubectl
			})

			// WARNING -- Find may return a bad value for higher trace levels
			routeRegexp := regexp.MustCompile(`https:\/\/.*sslip.io`)
			route := string(routeRegexp.Find([]byte(out)))

			Eventually(func() int {
				resp, err := env.Curl("GET", route, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				return resp.StatusCode
			}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

			By("deleting the app")
			env.DeleteApp(appName)

			By("checking the application resource was removed", func() {
				Eventually(func() string {
					out, _ := proc.Kubectl("get", "app",
						"--namespace", namespace, appName)
					return out
				}, "1m").Should(ContainSubstring("NotFound"))
			})
		})

		It("deploys an app from the current dir", func() {
			By("pushing the app in the current working directory")
			out := env.MakeApp(appName, 1, true)

			// WARNING -- Find may return a bad value for higher trace levels
			routeRegexp := regexp.MustCompile(`https:\/\/.*sslip.io`)
			route := string(routeRegexp.Find([]byte(out)))

			Eventually(func() int {
				resp, err := env.Curl("GET", route, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				return resp.StatusCode
			}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

			By("deleting the app")
			env.DeleteApp(appName)
		})

		It("deploys an app from the specified dir", func() {
			By("pushing the app in the specified app directory")
			env.MakeApp(appName, 1, false)

			By("deleting the app")
			env.DeleteApp(appName)
		})

		Context("manifest", func() {
			var manifestPath string

			origin := testenv.AssetPath("sample-app")

			BeforeEach(func() {
				manifestPath = catalog.NewTmpName("app.yml")
			})

			AfterEach(func() {
				err := os.Remove(manifestPath)
				Expect(err).ToNot(HaveOccurred())
			})

			It("deploys an app with the desired options", func() {
				By("providing a manifest")
				err := os.WriteFile(manifestPath, []byte(fmt.Sprintf(`origin:
  path: %s
name: %s
configuration:
  instances: 2
  environment:
    CREDO: up
    DOGMA: "no"
  appchart: standard
`, origin, appName)), 0600)
				Expect(err).ToNot(HaveOccurred())
				absManifestPath, err := filepath.Abs(manifestPath)
				Expect(err).ToNot(HaveOccurred())

				By("pushing the app specified in the manifest")

				out, err := env.EpinioPush("", appName, manifestPath)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Manifest: %s", absManifestPath))

				// TODO : Match push output lines ?

				By("verifying the stored settings")
				out, err = env.Epinio("", "app", "show", appName)
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(out).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Desired Instances", "2"),
						WithRow("App Chart", "standard"),
						WithRow("- CREDO", "up"),
						WithRow("- DOGMA", "no"),
					),
				)

				Expect(out).To(
					HaveATable(
						WithHeaders("NAME", "READY", "MEMORY", "MILLICPUS", "RESTARTS", "AGE"),
						WithRow("r"+appName+"-.*", "true", ".*", ".*", ".*", ".*"),
					),
				)

				By("deleting the app")
				env.DeleteApp(appName)
			})
		})

		It("removes the app's ingress when deleting an app", func() {
			env.MakeContainerImageApp(appName, 1, containerImageURL)

			By("deleting the app")
			env.DeleteApp(appName)

			Eventually(func() string {
				out, _ := proc.Kubectl("get", "ingress",
					"--namespace", namespace, appName)
				return out
			}, "1m").Should(ContainSubstring("not found"))

			Eventually(func() string {
				out, _ := proc.Kubectl("get", "service",
					"--namespace", namespace, appName)
				return out
			}, "1m").Should(ContainSubstring("not found"))
		})

		It("should not fail for a max-length application name", func() {
			appNameLong := "app123456789012345678901234567890123456789012345678901234567890"
			// 3+60 characters
			env.MakeContainerImageApp(appNameLong, 1, containerImageURL)

			By("deleting the app")
			env.DeleteApp(appNameLong)
		})

		It("should not fail for an application name with leading digits", func() {
			appNameLeadNumeric := "12monkeys"
			env.MakeContainerImageApp(appNameLeadNumeric, 1, containerImageURL)

			By("deleting the app")
			env.DeleteApp(appNameLeadNumeric)
		})

		It("respects the desired number of instances", func() {
			app := catalog.NewAppName()
			env.MakeContainerImageApp(app, 3, containerImageURL)
			defer env.DeleteApp(app)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "show", app)
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").Should(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Status", "3/3"),
				),
			)
		})

		It("deletes a batch of applications", func() {
			app1 := catalog.NewAppName()
			env.MakeContainerImageApp(app1, 1, containerImageURL)
			app2 := catalog.NewAppName()
			env.MakeContainerImageApp(app2, 1, containerImageURL)

			var out string
			var err error
			out, err = env.Epinio("", "app", "delete", app1, app2)
			Expect(err).ToNot(HaveOccurred(), out)

			Eventually(func() string {
				out, err = env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").ShouldNot(MatchRegexp("%s|%s", app1, app2))
		})

		Context("with configuration", func() {
			var configurationName string

			BeforeEach(func() {
				configurationName = catalog.NewConfigurationName()
				env.MakeConfiguration(configurationName)
			})

			AfterEach(func() {
				env.DeleteApp(appName)
				env.DeleteConfigurations(configurationName)
			})

			It("pushes an app with bound configurations", func() {
				currentDir, err := os.Getwd()
				Expect(err).ToNot(HaveOccurred())

				pushOutput, err := env.EpinioPush(path.Join(currentDir, "../assets/sample-app"),
					appName,
					"--name", appName,
					"--bind", configurationName)
				Expect(err).ToNot(HaveOccurred(), pushOutput)

				// And check presence
				Eventually(func() string {
					out, err := env.Epinio("", "app", "list")
					Expect(err).ToNot(HaveOccurred(), out)
					return out
				}, "2m").Should(
					HaveATable(
						WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
						WithRow(appName, WithDate(), "1/1", appName+".*", configurationName, ""),
					),
				)
			})
		})

		It("unbinds bound configurations when deleting an app, and then deletes the configuration", func() {
			configurationName := catalog.NewConfigurationName()

			env.MakeContainerImageApp(appName, 1, containerImageURL)
			env.MakeConfiguration(configurationName)
			env.BindAppConfiguration(appName, configurationName, namespace)

			By("deleting the app")
			out, err := env.Epinio("", "app", "delete", appName)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(
				HaveATable(
					WithHeaders("UNBOUND CONFIGURATIONS"),
					WithRow(configurationName),
				),
			)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").ShouldNot(ContainSubstring(appName))

			env.DeleteConfigurations(configurationName)
		})

		Context("with explicit domain secret", func() {
			var newDomainSecret string

			BeforeEach(func() {
				newDomainSecret = "domain" + appName
			})
			AfterEach(func() {
				env.DeleteApp(appName)

				out, err := proc.Kubectl("delete", "secret", "-n", namespace, "domain"+appName)
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("pushes an app", func() {
				By("Pushing an app normally")
				env.MakeContainerImageApp(appName, 1, containerImageURL)
				// During debugging this used SaveApplicationSpec and SaveServerLogs

				By("Getting the generated secret for the domain")
				// Actually only what is truly needed: pem data, and owning cert

				// query ingress for referenced secret
				ingSecret, err := proc.Kubectl("get", "ingress",
					"--namespace", namespace,
					"--selector", "app.kubernetes.io/name="+appName,
					"-o", "jsonpath={.items[*].spec.tls[*].secretName}")
				Expect(err).ToNot(HaveOccurred())

				// pull pem blocks out of the secret
				cao, err := proc.Kubectl("get", "secret", "-n", namespace, ingSecret, "-o", "jsonpath={.data['ca\\.crt']}")
				Expect(err).ToNot(HaveOccurred())

				crto, err := proc.Kubectl("get", "secret", "-n", namespace, ingSecret, "-o", "jsonpath={.data['tls\\.crt']}")
				Expect(err).ToNot(HaveOccurred())

				keyo, err := proc.Kubectl("get", "secret", "-n", namespace, ingSecret, "-o", "jsonpath={.data['tls\\.key']}")
				Expect(err).ToNot(HaveOccurred())

				// pull the owning cert resource out of the secret
				cert, err := proc.Kubectl("get", "secret", "-n", namespace, ingSecret, "-o", "jsonpath={.metadata.ownerReferences[*].name}")
				Expect(err).ToNot(HaveOccurred())

				// check that there is a cert
				out, err := proc.Kubectl("get", "certificate", "-n", namespace, cert, "-o", "json")
				Expect(err).ToNot(HaveOccurred(), out)

				// decode the pem blocks for insertion into the new secret
				ca, err := base64.StdEncoding.DecodeString(string(cao))
				Expect(err).ToNot(HaveOccurred(), string(cao))
				crt, err := base64.StdEncoding.DecodeString(string(crto))
				Expect(err).ToNot(HaveOccurred(), string(crto))
				key, err := base64.StdEncoding.DecodeString(string(keyo))
				Expect(err).ToNot(HaveOccurred(), string(keyo))

				By("Constructing a routing secret")

				directDomainSecret := &corev1.Secret{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Secret",
						APIVersion: "v1",
					},
					Type: "kubernetes.io/tls",
					ObjectMeta: metav1.ObjectMeta{
						Name:      newDomainSecret,
						Namespace: namespace,
						Labels: map[string]string{
							"epinio.io/routing": "domain-mapping-test",
						},
					},
					StringData: map[string]string{
						"ca.crt":  string(ca),
						"tls.crt": string(crt),
						"tls.key": string(key),
					},
				}

				domainFile := catalog.NewTmpName("tmpUserFile") + `.json`
				file, err := os.Create(domainFile)
				Expect(err).ToNot(HaveOccurred())

				err = json.NewEncoder(file).Encode(directDomainSecret)
				Expect(err).ToNot(HaveOccurred())
				defer os.Remove(domainFile)

				By("Uploading the new secret for the domain")

				out, err = proc.Kubectl("apply", "-f", domainFile)
				Expect(err).ToNot(HaveOccurred(), out)

				// check success of upload, new secret should exist
				new, err := proc.Kubectl("get", "secret", "-n", namespace, newDomainSecret, "-o", "json")
				Expect(err).ToNot(HaveOccurred(), new)

				By("Pushing the app again")
				env.MakeContainerImageApp(appName, 1, containerImageURL)
				// During debugging this used SaveApplicationSpec and SaveServerLogs

				By("Seeing the generated cert gone")

				// check that the generated cert from the first push is gone
				out, err = proc.Kubectl("get", "certificate", "-n", namespace, cert, "-o", "json")
				Expect(err).To(HaveOccurred(), out)

				By("Seeing the generated secret gone")

				// check that the generated secret from the first push is gone
				out, err = proc.Kubectl("get", "secret", "-n", namespace, ingSecret, "-o", "json")
				Expect(err).To(HaveOccurred(), out)

				By("Seeing the ingress use the new routing secret")

				// check the secret referenced by the updated app ingress, should be the new
				newSecret, err := proc.Kubectl("get", "ingress",
					"--namespace", namespace,
					"--selector", "app.kubernetes.io/name="+appName,
					"-o", "jsonpath={.items[*].spec.tls[*].secretName}")
				Expect(err).ToNot(HaveOccurred())
				Expect(newSecret).To(Equal(newDomainSecret))
			})
		})

		Context("with environment variable", func() {
			AfterEach(func() {
				env.DeleteApp(appName)
			})

			It("pushes an app", func() {
				currentDir, err := os.Getwd()
				Expect(err).ToNot(HaveOccurred())

				pushOutput, err := env.EpinioPush(path.Join(currentDir, "../assets/sample-app"),
					appName,
					"--name", appName,
					"--env", "MYVAR=myvalue")
				Expect(err).ToNot(HaveOccurred(), pushOutput)

				// And check presence
				Eventually(func() string {
					out, err := env.Epinio("", "apps", "env", "list", appName)
					Expect(err).ToNot(HaveOccurred(), out)
					return out
				}, "2m").Should(
					HaveATable(
						WithHeaders("VARIABLE", "VALUE"),
						WithRow("MYVAR", "myvalue"),
					),
				)
			})
		})
	})

	Describe("update", func() {
		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("respects the desired number of instances", func() {
			env.MakeContainerImageApp(appName, 1, containerImageURL)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "show", appName)
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").Should(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Status", "1/1"),
				),
			)

			out, err := env.Epinio("", "app", "update", appName, "-i", "3")
			Expect(err).ToNot(HaveOccurred(), out)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "show", appName)
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").Should(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Status", "3/3"),
				),
			)
		})

		Context("with configuration", func() {
			var configurationName string

			BeforeEach(func() {
				configurationName = catalog.NewConfigurationName()
				env.MakeConfiguration(configurationName)
			})

			AfterEach(func() {
				env.UnbindAppConfiguration(appName, configurationName, namespace)
				env.DeleteConfigurations(configurationName)
				// DeleteApp see outer context
			})

			It("respects the bound configurations", func() {
				env.MakeContainerImageApp(appName, 1, containerImageURL)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

					return out
				}, "1m").Should(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Status", "1/1"),
					),
				)

				out, err := env.Epinio("", "app", "update", appName, "--bind", configurationName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Successfully updated application"))

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

					return out
				}, "1m").Should(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Bound Configurations", configurationName),
					),
				)
			})
		})
	})

	Describe("list, show, and export", func() {
		var configurationName string
		BeforeEach(func() {
			configurationName = catalog.NewConfigurationName()
			env.MakeContainerImageApp(appName, 1, containerImageURL)
			env.MakeConfiguration(configurationName)
			env.BindAppConfiguration(appName, configurationName, namespace)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
			env.CleanupConfiguration(configurationName)
		})

		It("lists all apps in the namespace", func() {
			out, err := env.Epinio("", "app", "list")
			Expect(err).ToNot(HaveOccurred(), out)

			By(out)

			Expect(out).To(ContainSubstring("Listing applications"))
			Expect(out).To(
				HaveATable(
					WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(appName, WithDate(), "(1|2)/1", appName+".*", configurationName, ".*"),
				),
			)
		})

		It("lists all apps in the namespace in JSON format", func() {
			out, err := env.Epinio("", "app", "list", "--output", "json")
			Expect(err).ToNot(HaveOccurred(), out)

			apps := models.AppList{}
			err = json.Unmarshal([]byte(out), &apps)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(apps).ToNot(BeEmpty())
		})

		It("shows the details of an app", func() {
			out, err := env.Epinio("", "app", "show", appName)
			Expect(err).ToNot(HaveOccurred(), out)

			By(out)

			Expect(out).To(ContainSubstring("Show application details"))
			Expect(out).To(ContainSubstring("Application: %s", appName))

			Expect(out).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Origin", containerImageURL),
					WithRow("Bound Configurations", configurationName),
					WithRow("Active Routes", ""),
					WithRow("", appName+".*"),
				),
			)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "show", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").Should(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Status", "1/1"),
				),
			)
		})

		Context("details customized", func() {
			var chartName string
			var appName string
			var tempFile string

			BeforeEach(func() {
				chartName = catalog.NewTmpName("chart-")
				tempFile = env.MakeAppchart(chartName)

				appName = catalog.NewAppName()
				out, err := env.Epinio("", "app", "create", appName,
					"--app-chart", chartName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("Ok"))
			})

			AfterEach(func() {
				env.DeleteApp(appName)
				env.DeleteAppchart(tempFile)
			})

			It("shows the details of a customized app", func() {
				out, err := env.Epinio("", "app", "update", appName,
					"--chart-value", "foo=bar")
				Expect(err).ToNot(HaveOccurred(), out)

				out, err = env.Epinio("", "app", "show", appName)
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(out).To(ContainSubstring("Show application details"))
				Expect(out).To(ContainSubstring("Application: %s", appName))

				Expect(out).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Origin", "<<undefined>>"),
						WithRow("App Chart", chartName),
						WithRow("Chart Values", ""),
						WithRow("- foo", "bar"),
					),
				)
			})

			Context("exporting customized", func() {
				var domain, chartName, tempFile, app, exportPath, exportValues, exportChart, exportImage string

				BeforeEach(func() {
					domain = catalog.NewTmpName("exportdomain-") + ".org"
					chartName = catalog.NewTmpName("chart-")
					tempFile = env.MakeAppchart(chartName)

					app = catalog.NewAppName()

					exportPath = catalog.NewTmpName(app + "-export")
					exportValues = path.Join(exportPath, "values.yaml")
					exportChart = path.Join(exportPath, "app-chart.tar.gz")
					exportImage = path.Join(exportPath, "app-image.tar")

					env.MakeRoutedContainerImageApp(app, 1, containerImageURL, domain,
						"--app-chart", chartName,
						"--chart-value", "foo=bar",
					)
				})

				AfterEach(func() {
					env.DeleteApp(app)
					env.DeleteAppchart(tempFile)

					err := os.RemoveAll(exportPath)
					Expect(err).ToNot(HaveOccurred())
				})

				It("fails to export on conflict between destinations", func() {
					out, err := env.Epinio("", "app", "export", app, exportPath, "--registry", "__bogus__")
					Expect(err).To(HaveOccurred(), out)
					Expect(out).To(ContainSubstring("Conflict, both directory and registry destinations found"))
				})

				It("fails to export without a destination", func() {
					out, err := env.Epinio("", "app", "export", app)
					Expect(err).To(HaveOccurred(), out)
					Expect(out).To(ContainSubstring("Neither directory nor registry destination found"))
				})

				It("fails to export for an unknown registry destination", func() {
					out, err := env.Epinio("", "app", "export", app, "--registry", "__bogus__")
					Expect(err).To(HaveOccurred(), out)
					Expect(out).To(ContainSubstring("bad export destination"))
				})

				It("fails to export an unknown application", func() {
					out, err := env.Epinio("", "app", "export", "__bogus__", "--registry", "foo")
					Expect(err).To(HaveOccurred(), out)
					Expect(out).To(ContainSubstring(""))
				})

				It("exports the details of a customized app", func() {
					out, err := env.Epinio("", "app", "export", app, exportPath)
					Expect(err).ToNot(HaveOccurred(), out)

					exported, err := filepath.Glob(exportPath + "/*")
					Expect(err).ToNot(HaveOccurred(), exported)
					Expect(exported).To(ConsistOf([]string{exportValues, exportChart, exportImage}))

					Expect(exportPath).To(BeADirectory())
					Expect(exportValues).To(BeARegularFile())
					Expect(exportChart).To(BeARegularFile())
					Expect(exportImage).To(BeARegularFile())

					values, err := os.ReadFile(exportValues)
					Expect(err).ToNot(HaveOccurred(), string(values))

					Expect(string(values)).To(Equal(fmt.Sprintf(`chartConfig:
  tuning: speed
epinio:
  appName: %s
  configpaths: []
  configurations: []
  env: []
  imageURL: epinio/sample-app
  ingress: null
  replicaCount: 1
  routes:
  - domain: %s
    id: %s
    path: /
  stageID: ""
  start: null
  tlsIssuer: epinio-ca
  username: admin
userConfig:
  foo: bar
`, app, domain, domain)))
					// Not checking that exportChart is a proper tarball.
				})
			})
		})

		Context("exporting", func() {
			var domain, app, exportPath, exportValues, exportChart, exportImage string

			BeforeEach(func() {
				domain = catalog.NewTmpName("exportdomain-") + ".org"
				app = catalog.NewAppName()

				exportPath = catalog.NewTmpName(app + "-export")
				exportValues = path.Join(exportPath, "values.yaml")
				exportChart = path.Join(exportPath, "app-chart.tar.gz")
				exportImage = path.Join(exportPath, "app-image.tar")

				env.MakeRoutedContainerImageApp(app, 1, containerImageURL, domain)
			})

			AfterEach(func() {
				env.DeleteApp(app)

				err := os.RemoveAll(exportPath)
				Expect(err).ToNot(HaveOccurred())
			})

			It("exports the details of an app", func() {
				out, err := env.Epinio("", "app", "export", app, exportPath)
				Expect(err).ToNot(HaveOccurred(), out)

				exported, err := filepath.Glob(exportPath + "/*")
				Expect(err).ToNot(HaveOccurred(), exported)
				Expect(exported).To(ConsistOf([]string{exportValues, exportChart, exportImage}))

				Expect(exportPath).To(BeADirectory())
				Expect(exportValues).To(BeARegularFile())
				Expect(exportChart).To(BeARegularFile())
				Expect(exportImage).To(BeARegularFile())

				values, err := os.ReadFile(exportValues)
				Expect(err).ToNot(HaveOccurred(), string(values))
				Expect(string(values)).To(Equal(fmt.Sprintf(`epinio:
  appName: %s
  configpaths: []
  configurations: []
  env: []
  imageURL: epinio/sample-app
  ingress: null
  replicaCount: 1
  routes:
  - domain: %s
    id: %s
    path: /
  stageID: ""
  start: null
  tlsIssuer: epinio-ca
  username: admin
`, app, domain, domain)))
				// Not checking that exportChart is a proper tarball.
			})

			It("correctly handles complex quoting when deploying and exporting an app", func() {
				out, err := env.Epinio("", "apps", "env", "set", app,
					"complex", `{
   "usernameOrOrg": "scures",
   "url":           "https://github.com/scures/epinio-sample-app",
   "commit":        "3ce7abe14abd849b374eb68729de8c71e9f3a927"
}`)
				Expect(err).ToNot(HaveOccurred(), out)

				out, err = env.Epinio("", "app", "export", app, exportPath)
				Expect(err).ToNot(HaveOccurred(), out)

				exported, err := filepath.Glob(exportPath + "/*")
				Expect(err).ToNot(HaveOccurred(), exported)
				Expect(exported).To(ConsistOf([]string{exportValues, exportChart, exportImage}))

				Expect(exportPath).To(BeADirectory())
				Expect(exportValues).To(BeARegularFile())
				Expect(exportChart).To(BeARegularFile())
				Expect(exportImage).To(BeARegularFile())

				values, err := os.ReadFile(exportValues)
				Expect(err).ToNot(HaveOccurred(), string(values))

				Expect(string(values)).To(Equal(fmt.Sprintf(`epinio:
  appName: %s
  configpaths: []
  configurations: []
  env:
  - name: complex
    value: |-
      {
         "usernameOrOrg": "scures",
         "url":           "https://github.com/scures/epinio-sample-app",
         "commit":        "3ce7abe14abd849b374eb68729de8c71e9f3a927"
      }
  imageURL: epinio/sample-app
  replicaCount: 1
  routes:
  - domain: %s
    id: %s
    path: /
  stageID: ""
  tlsIssuer: epinio-ca
  username: admin
`, app, domain, domain)))
				// Not checking that exportChart is a proper tarball.
			})
		})

		Describe("no instances", func() {
			// Note to test maintainers. This sections pushes an app with zero instances
			// to begin with. This avoids termination issues we have seen, where the pod
			// termination invoked when scaling down to 0 takes a very long time (over
			// two minutes).

			var app string

			BeforeEach(func() {
				app = catalog.NewAppName()
				By("make zero-instance app: " + app)
				env.MakeApp(app, 0, false)
				By("pushed")
			})

			AfterEach(func() {
				By("delete app")
				env.DeleteApp(app)
				By("deleted")
			})

			It("lists apps without instances", func() {
				By("list apps")
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(
					HaveATable(
						WithHeaders("NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
						WithRow(app, WithDate(), "0/0", "n/a", "", ""),
					),
				)
			})

			It("shows the details of an app without instances", func() {
				By("show details")
				out, err := env.Epinio("", "app", "show", app)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Status", "deployed, scaled to zero"),
					),
				)
			})

			It("scales up to a running workload", func() {
				out, err := env.Epinio("", "app", "update", app, "-i", "3")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", app)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
					return out
				}, "1m").Should(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Status", "3/3"),
					),
				)
			})
		})
	})

	Describe("list across namespaces", func() {
		var namespace1 string
		var namespace2 string
		var app1 string
		var app2 string

		BeforeEach(func() {
			namespace1 = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace1)

			app1 = catalog.NewAppName()
			env.MakeContainerImageApp(app1, 1, containerImageURL)

			namespace2 = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace2)

			app2 = catalog.NewAppName()
			env.MakeContainerImageApp(app2, 1, containerImageURL)
		})

		AfterEach(func() {
			env.TargetNamespace(namespace2)
			env.DeleteApp(app2)
			env.DeleteNamespace(namespace2)

			env.TargetNamespace(namespace1)
			env.DeleteApp(app1)
			env.DeleteNamespace(namespace1)
		})

		It("lists all applications belonging to all namespaces", func() {
			// But we care only about the two we know about from the setup.

			out, err := env.Epinio("", "app", "list", "--all")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Listing all applications"))

			By(out)

			Expect(out).To(
				HaveATable(
					WithHeaders("NAMESPACE", "NAME", "CREATED", "STATUS", "ROUTES", "CONFIGURATIONS", "STATUS DETAILS"),
					WithRow(namespace1, app1, WithDate(), "1/1", app1+".*", "", ".*"),
					WithRow(namespace2, app2, WithDate(), "1/1", app2+".*", "", ".*"),
				),
			)
		})
	})

	Describe("logs", func() {
		var (
			route     string
			logLength int
		)

		BeforeEach(func() {
			By("deploying an app")

			out := env.MakeApp(appName, 1, true)
			routeRegexp := regexp.MustCompile(`https:\/\/.*sslip.io`)
			route = string(routeRegexp.Find([]byte(out)))

			By("getting the current logs in full")
			out, err := env.Epinio("", "app", "logs", appName)
			Expect(err).ToNot(HaveOccurred(), out)

			podNames := env.GetPodNames(appName, namespace)
			for _, podName := range podNames {
				Expect(out).To(ContainSubstring(podName))
			}
			logs := strings.Split(out, "\n")
			logLength = len(logs)

			By("----------------------------------")
			By(fmt.Sprintf("LOGS = %d lines (raw)", logLength))

			for idx, line := range logs {
				// Exclude fake log lines caused by coverage collection.
				if (line == "PASS") || strings.Contains(line, "coverage") {
					logLength--
					continue
				}
				By(fmt.Sprintf("LOG_ [%3d]: %s", idx, line))
			}
			By("----------------------------------")
			By(fmt.Sprintf("LOGS = %d lines (filtered)", logLength))

			// Skip correction (coverage, if present, is already accounted for, see above)
			logLength = logLength - 1
			By(fmt.Sprintf("SKIP %d lines", logLength))
		})

		AfterEach(func() {
			By("removing the app")
			env.DeleteApp(appName)
		})

		It("shows the staging logs", func() {
			out, err := env.Epinio("", "app", "logs", "--staging", appName)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(ContainSubstring(`Generating default PHP configuration`))
			// Doesn't include linkerd sidecar logs
			Expect(out).ToNot(ContainSubstring(`linkerd-`))
		})

		It("follows logs", func() {
			p, err := proc.Get("", testenv.EpinioBinaryPath(), "app", "logs", "--follow", appName)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				if p.Process != nil {
					p.Process.Kill()
				}
			}()
			reader, err := p.StdoutPipe()
			Expect(err).NotTo(HaveOccurred())
			go p.Run()

			By("read all the logs")
			scanner := bufio.NewScanner(reader)

			By("get to the end of logs")
			By("----------------------------------")
			for i := 0; i < logLength; i++ {
				By(fmt.Sprintf("SCAN [%3d]", i))
				scanner.Scan()
				By(fmt.Sprintf("SKIP [%3d]: %s", i, scanner.Text()))
			}
			By("----------------------------------")

			By("adding new logs")
			// Theoretically "Eventually" shouldn't be required. http 200 should be
			// returned on the first try. This test flaked here, sometimes returning
			// 404. We are suspecting some bug in k3d networking which made the Ingress
			// return 404 if accessed too quickly.
			Eventually(func() int {
				resp, err := env.Curl("GET", route, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				return resp.StatusCode
			}, "1m").Should(Equal(http.StatusOK))

			By("checking the latest log")
			Eventually(func() string {
				scanner.Scan()
				return scanner.Text()
			}, "30s").Should((ContainSubstring("[200]: GET /")))
		})
	})

	Describe("exec", func() {
		BeforeEach(func() {
			pushOutput, err := env.Epinio("", "apps", "push",
				"--name", appName,
				"--container-image-url", containerImageURL,
			)
			Expect(err).ToNot(HaveOccurred(), pushOutput)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("executes a command in the application's container (one of the pods)", func() {
			var out bytes.Buffer
			containerCmd := bytes.NewReader([]byte("echo testthis > /workspace/testfile && exit\r"))

			cmd := exec.Command(testenv.EpinioBinaryPath(), "apps", "exec", appName)
			cmd.Stdin = containerCmd
			cmd.Stdout = &out
			cmd.Stderr = &out

			err := cmd.Run()
			Expect(err).ToNot(HaveOccurred())

			Expect(out.String()).To(ContainSubstring("Executing a shell"))

			podName, err := proc.Kubectl("get", "pods",
				"-l", fmt.Sprintf("app.kubernetes.io/name=%s", appName),
				"-n", namespace, "-o", "name")
			Expect(err).ToNot(HaveOccurred())

			remoteOut, err := proc.Kubectl("exec",
				strings.TrimSpace(podName), "-n", namespace,
				"--", "cat", "/workspace/testfile")
			Expect(err).ToNot(HaveOccurred(), remoteOut)

			// The command we run should have effects
			Expect(strings.TrimSpace(remoteOut)).To(Equal("testthis"))
		})
	})

	When("pushing an app with a numeric-only name", func() {
		BeforeEach(func() {
			min := 9000
			max := 10000
			randNum := r.Intn(max-min+1) + min
			appName = strconv.Itoa(randNum)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("deploys successfully", func() {
			pushOutput, err := env.Epinio("", "apps", "push",
				"--name", appName,
				"--container-image-url", containerImageURL,
			)
			Expect(err).ToNot(HaveOccurred(), pushOutput)
		})
	})

	for _, command := range []string{
		"exec",
		"export",
		"logs",
		"manifest",
		"port-forward",
		"restage",
		"restart",
		"show",
		"update",
		"delete",
	} {
		Context(command+" command completion", func() {
			BeforeEach(func() {
				out, err := env.Epinio("", "app", "create", appName)
				Expect(err).ToNot(HaveOccurred(), out)
			})

			AfterEach(func() {
				env.DeleteApp(appName)
			})

			It("matches empty prefix", func() {
				out, err := env.Epinio("", "__complete", "app", command, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(appName))
			})

			It("does not match unknown prefix", func() {
				out, err := env.Epinio("", "__complete", "app", command, "bogus")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring("bogus"))
			})

			It("does not match bogus arguments", func() {
				out, err := env.Epinio("", "__complete", "app", command, appName, "")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).ToNot(ContainSubstring(appName))
			})
		})
	}

	var _ = Describe("Custom chart-value", Label("appListeningPort"), func() {
		var (
			namespace string
			appName   string
		)

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			appName = catalog.NewAppName()
		})

		AfterEach(func() {
			env.DeleteNamespace(namespace)
		})

		Context("with chart-value:", func() {
			AfterEach(func() {
				env.DeleteApp(appName)
			})

			It("appListeningPort, pushes an app", func() {
				currentDir, err := os.Getwd()
				Expect(err).ToNot(HaveOccurred())

				appListeningPort := ""

				if port := r.Intn(65536); port <= 1023 {
					appListeningPort = fmt.Sprintf("%d", 80)
				} else {
					appListeningPort = fmt.Sprintf("%d", port)
				}

				pushOutput, err := env.EpinioPush(path.Join(currentDir, "../assets/sample-app"),
					appName,
					"--name", appName,
					"--chart-value", "appListeningPort="+appListeningPort)
				Expect(err).ToNot(HaveOccurred(), pushOutput)

				out, err := proc.Kubectl("get", "app",
					"--namespace", namespace, appName,
					"-o", "jsonpath={.spec.settings.appListeningPort}")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(out).To(Equal(appListeningPort))

				out, err = proc.Kubectl("get", "pod",
					"--namespace", namespace,
					"-o", "jsonpath={.items[0].spec.containers[0].env[0].value}")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(out).To(Equal(appListeningPort))

				out, err = proc.Kubectl("get", "pod",
					"--namespace", namespace,
					"-o", "jsonpath={.items[0].spec.containers[0].ports[0].containerPort}")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(out).To(Equal(appListeningPort))

				out, err = proc.Kubectl("get", "svc",
					"--namespace", namespace,
					"-o", "jsonpath={.items[0].spec.ports[0].targetPort}")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(out).To(Equal(appListeningPort))
			})
		})

		Context("without chart-value:", func() {
			AfterEach(func() {
				env.DeleteApp(appName)
			})

			It("appListeningPort, pushes an app", func() {
				currentDir, err := os.Getwd()
				Expect(err).ToNot(HaveOccurred())

				appListeningPort := "8080"

				pushOutput, err := env.EpinioPush(path.Join(currentDir, "../assets/sample-app"),
					appName,
					"--name", appName)
				Expect(err).ToNot(HaveOccurred(), pushOutput)

				out, err := proc.Kubectl("get", "pod",
					"--namespace", namespace,
					"-o", "jsonpath={.items[0].spec.containers[0].env[0].value}")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(out).To(Equal(appListeningPort))

				out, err = proc.Kubectl("get", "pod",
					"--namespace", namespace,
					"-o", "jsonpath={.items[0].spec.containers[0].ports[0].containerPort}")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(out).To(Equal(appListeningPort))

				out, err = proc.Kubectl("get", "svc",
					"--namespace", namespace,
					"-o", "jsonpath={.items[0].spec.ports[0].targetPort}")
				Expect(err).NotTo(HaveOccurred(), out)
				Expect(out).To(Equal(appListeningPort))
			})
		})
	})
})
