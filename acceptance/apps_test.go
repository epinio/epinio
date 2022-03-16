package acceptance_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps", func() {
	var (
		namespace string
		appName   string
	)
	containerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		appName = catalog.NewAppName()
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	When("creating an application without a workload", func() {
		AfterEach(func() {
			// MakeApp... by each test (It)
			env.DeleteApp(appName)
		})

		It("creates the app", func() {
			out, err := env.Epinio("", "app", "create", appName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Ok"))
		})

		Context("with configuration", func() {
			var configurationName string

			BeforeEach(func() {
				configurationName = catalog.NewConfigurationName()
				env.MakeConfiguration(configurationName)
			})

			AfterEach(func() {
				env.DeleteConfigurationUnbind(configurationName)
				// env.DeleteApp see outer context
			})

			It("creates the app with instance count, configurations, and environment", func() {
				out, err := env.Epinio("", "app", "create", appName,
					"--bind", configurationName,
					"--instances", "2",
					"--env", "CREDO=up",
					"--env", "DOGMA=no")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp("Ok"))

				out, err = env.Epinio("", "app", "show", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp(`Instances\s*\|\s*2\s*\|`))
				Expect(out).To(MatchRegexp(`Configurations\s*\|\s*` + configurationName + `\s*\|`))
				Expect(out).To(MatchRegexp(`- CREDO\s*\|\s*up\s*\|`))
				Expect(out).To(MatchRegexp(`- DOGMA\s*\|\s*no\s*\|`))
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
						"--bind", configurationName,
						"--instances", "2",
						"--env", "CREDO=up",
						"--env", "DOGMA=no")
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).To(MatchRegexp("Ok"))

					out, err = env.Epinio("", "app", "manifest", appName, destinationPath)
					Expect(err).ToNot(HaveOccurred(), out)

					manifest, err := ioutil.ReadFile(destinationPath)
					Expect(err).ToNot(HaveOccurred(), destinationPath)

					Expect(string(manifest)).To(MatchRegexp(fmt.Sprintf(`name: %s
configuration:
  instances: 2
  configurations:
  - %s
  environment:
    CREDO: up
    DOGMA: "no"
  routes:
  - %s\..*
`, appName, configurationName, appName)))
				})
			})
		})

		It("creates the app with environment variables", func() {
			out, err := env.Epinio("", "app", "create", appName, "--env", "MYVAR=myvalue")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Ok"))

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
				out, err := env.EpinioPush(appDir, appName, "--name", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("App is online"))
			})
		})
	})

	When("pushing an app from an external repository", func() {
		It("pushes the app successfully", func() {
			wordpress := "https://github.com/epinio/example-wordpress"
			pushLog, err := env.EpinioPush("",
				appName,
				"--name", appName,
				"--git", wordpress+",main")
			Expect(err).ToNot(HaveOccurred(), pushLog)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "5m").Should(MatchRegexp(fmt.Sprintf(`%s.*\|.*1\/1.*\|.*`, appName)))

			By("deleting the app")
			env.DeleteApp(appName)
		})

		Describe("update", func() {
			BeforeEach(func() {
				wordpress := "https://github.com/epinio/example-wordpress"
				pushLog, err := env.EpinioPush("", appName, "--name", appName, "--git", wordpress+",main")
				Expect(err).ToNot(HaveOccurred(), pushLog)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "list")
					Expect(err).ToNot(HaveOccurred(), out)
					return out
				}, "5m").Should(MatchRegexp(fmt.Sprintf(`%s.*\|.*1\/1.*\|.*`, appName)))

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

					return out
				}, "1m").Should(MatchRegexp(`Status\s*\|\s*1\/1\s*\|`))
			})

			It("respects the desired number of instances", func() {
				out, err := env.Epinio("", "app", "update", appName, "-i", "3")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

					return out
				}, "1m").Should(MatchRegexp(`Status\s*\|\s*3\/3\s*\|`))
			})

			It("respects environment variable changes", func() {
				out, err := env.Epinio("", "app", "update", appName, "--env", "MYVAR=myvalue")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					out, err := env.Epinio("", "apps", "env", "list", appName)
					Expect(err).ToNot(HaveOccurred(), out)
					return out
				}, "2m").Should(MatchRegexp("MYVAR.*myvalue"))
			})

			AfterEach(func() {
				env.DeleteApp(appName)
			})
		})
	})

	Describe("restage", func() {
		When("restaging an existing app", func() {
			It("will be staged again", func() {
				env.MakeApp(appName, 1, false)

				restageLogs, err := env.Epinio("", "app", "restage", appName)
				Expect(err).ToNot(HaveOccurred(), restageLogs)

				By("deleting the app")
				env.DeleteApp(appName)
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
				Expect(restageLogs).Should(MatchRegexp("Unable to restage container-based application"))

				By("deleting the app")
				env.DeleteApp(appName)
			})
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
				"--builder-image", "paketobuildpacks/builder:tiny")
			Expect(err).ToNot(HaveOccurred(), pushLog)

			By("checking if the staging is using custom builder image")
			labels := fmt.Sprintf("app.kubernetes.io/name=%s,app.kubernetes.io/component=staging", appName)
			imageList, err := proc.Kubectl("get", "pod",
				"--namespace", testenv.Namespace,
				"-l", labels,
				"-o", "jsonpath={.items[0].spec.containers[*].image}")
			Expect(err).NotTo(HaveOccurred())
			Expect(imageList).To(ContainSubstring("paketobuildpacks/builder:tiny"))
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
			n, err := proc.Kubectl("get", "deployment",
				"--namespace", ns, name,
				"-o", "jsonpath={.spec.replicas}")
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

				Expect(out).To(MatchRegexp("Reusing cache layer"))
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
				Expect(out).To(MatchRegexp(fmt.Sprintf(`persistentvolumeclaims "%s" not found`, names.GenerateResourceName(namespace, appName))))
			})
		})
	})

	Describe("push and delete", func() {
		It("shows the staging logs", func() {
			By("pushing the app")
			out := env.MakeApp(appName, 1, true)

			Expect(out).To(MatchRegexp(`.*Configuring PHP Application.*`))
			Expect(out).To(MatchRegexp(`.*Using feature -- PHP.*`))
			// Doesn't include linkerd sidecar logs
			Expect(out).ToNot(MatchRegexp(`linkerd-.*`))
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
			routeRegexp := regexp.MustCompile(`https:\/\/.*omg.howdoi.website`)
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
			routeRegexp := regexp.MustCompile(`https:\/\/.*omg.howdoi.website`)
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
				err := ioutil.WriteFile(manifestPath, []byte(fmt.Sprintf(`origin:
  path: %s
name: %s
configuration:
  instances: 2
  environment:
    CREDO: up
    DOGMA: "no"
`, origin, appName)), 0600)
				Expect(err).ToNot(HaveOccurred())
				absManifestPath, err := filepath.Abs(manifestPath)
				Expect(err).ToNot(HaveOccurred())

				By("pushing the app specified in the manifest")

				out, err := env.EpinioPush("", appName, manifestPath)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp(`Manifest: ` + absManifestPath))

				// TODO : Match push output lines ?

				By("verifying the stored settings")
				out, err = env.Epinio("", "app", "show", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp(`Instances\s*\|\s*2\s*\|`))
				Expect(out).To(MatchRegexp(`- CREDO\s*\|\s*up\s*\|`))
				Expect(out).To(MatchRegexp(`- DOGMA\s*\|\s*no\s*\|`))
				Expect(out).To(MatchRegexp(fmt.Sprintf(" %s-.*|\\s+true\\s+|.*|.*|.*|.*|", appName)))

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
			}, "1m").Should(MatchRegexp(`Status\s*\|\s*3\/3\s*\|`))
		})

		Context("with configuration", func() {
			var configurationName string

			BeforeEach(func() {
				configurationName = catalog.NewConfigurationName()
				env.MakeConfiguration(configurationName)
			})

			AfterEach(func() {
				env.DeleteApp(appName)
				env.DeleteConfiguration(configurationName)
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
				}, "2m").Should(MatchRegexp(appName + `.*\|.*1\/1.*\|.*` + configurationName))
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

			Expect(out).To(MatchRegexp("UNBOUND CONFIGURATIONS"))
			Expect(out).To(MatchRegexp(configurationName))

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").ShouldNot(MatchRegexp(`.*%s.*`, appName))

			env.DeleteConfiguration(configurationName)
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
				}, "2m").Should(MatchRegexp("MYVAR.*myvalue"))
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
			}, "1m").Should(MatchRegexp(`Status\s*\|\s*1\/1\s*\|`))

			out, err := env.Epinio("", "app", "update", appName, "-i", "3")
			Expect(err).ToNot(HaveOccurred(), out)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "show", appName)
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").Should(MatchRegexp(`Status\s*\|\s*3\/3\s*\|`))
		})

		Context("with configuration", func() {
			var configurationName string

			BeforeEach(func() {
				configurationName = catalog.NewConfigurationName()
				env.MakeConfiguration(configurationName)
			})

			AfterEach(func() {
				env.UnbindAppConfiguration(appName, configurationName, namespace)
				env.DeleteConfiguration(configurationName)
				// DeleteApp see outer context
			})

			It("respects the bound configurations", func() {
				env.MakeContainerImageApp(appName, 1, containerImageURL)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

					return out
				}, "1m").Should(MatchRegexp(`Status\s*\|\s*1\/1\s*\|`))

				out, err := env.Epinio("", "app", "update", appName, "--bind", configurationName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp("Successfully updated application"))

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

					return out
				}, "1m").Should(MatchRegexp(`Configurations\s*\|\s*` + configurationName + `\s*\|`))
			})
		})
	})

	Describe("list and show", func() {
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
			Expect(out).To(MatchRegexp("Listing applications"))
			Expect(out).To(MatchRegexp(" " + appName + " "))
			Expect(out).To(MatchRegexp(" " + configurationName + " "))
		})

		It("shows the details of an app", func() {
			out, err := env.Epinio("", "app", "show", appName)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Show application details"))
			Expect(out).To(MatchRegexp("Application: " + appName))
			Expect(out).To(MatchRegexp(`Origin .*\|.* ` + containerImageURL))
			Expect(out).To(MatchRegexp(`Configurations .*\|.* ` + configurationName))
			Expect(out).To(MatchRegexp("Routes .*\n|.* " + appName))

			Eventually(func() string {
				out, err := env.Epinio("", "app", "show", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").Should(MatchRegexp(`Status .*\|.* 1\/1`))
		})

		Describe("no instances", func() {
			BeforeEach(func() {
				out, err := env.Epinio("", "app", "update", appName, "--instances", "0")
				Expect(err).ToNot(HaveOccurred(), out)
			})
			It("lists apps without instances", func() {
				Eventually(func() string {
					out, err := env.Epinio("", "app", "list")
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
					return out
				}, "1m").Should(MatchRegexp("0/0"))
			})
			It("shows the details of an app without instances", func() {
				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

					return out
				}, "1m").Should(MatchRegexp(`Status\s*\|\s*0\/0\s*\|`))
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
			Expect(out).To(MatchRegexp("Listing all applications"))

			Expect(out).To(MatchRegexp(" " + app1 + " "))
			Expect(out).To(MatchRegexp(" " + app2 + " "))
		})
	})

	Describe("logs", func() {
		var (
			route     string
			logLength int
		)

		BeforeEach(func() {
			out := env.MakeApp(appName, 1, true)
			routeRegexp := regexp.MustCompile(`https:\/\/.*omg.howdoi.website`)
			route = string(routeRegexp.Find([]byte(out)))

			out, err := env.Epinio("", "app", "logs", appName)
			Expect(err).ToNot(HaveOccurred(), out)

			podNames := env.GetPodNames(appName, namespace)
			for _, podName := range podNames {
				Expect(out).To(ContainSubstring(podName))
			}
			logs := strings.Split(out, "\n")
			logLength = len(logs)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("shows the staging logs", func() {
			out, err := env.Epinio("", "app", "logs", "--staging", appName)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp(`.*Configuring PHP Application.*`))
			Expect(out).To(MatchRegexp(`.*Using feature -- PHP.*`))
			// Doesn't include linkerd sidecar logs
			Expect(out).ToNot(MatchRegexp(`linkerd-.*`))
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
			for i := 0; i < logLength-1; i++ {
				scanner.Scan()
				scanner.Text()
			}

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
			scanner.Scan()
			Expect(scanner.Text()).To(ContainSubstring("GET / HTTP/1.1"))
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

			Expect(out.String()).To(MatchRegexp("Executing a shell"))

			podName, err := proc.Kubectl("get", "pods",
				"-l", fmt.Sprintf("app.kubernetes.io/name=%s", appName),
				"-n", namespace, "-o", "name")
			Expect(err).ToNot(HaveOccurred())

			remoteOut, err := proc.Kubectl("exec",
				strings.TrimSpace(podName), "-n", namespace, "-c", appName,
				"--", "cat", "/workspace/testfile")
			Expect(err).ToNot(HaveOccurred())

			// The command we run should have effects
			Expect(strings.TrimSpace(remoteOut)).To(Equal("testthis"))
		})
	})
})
