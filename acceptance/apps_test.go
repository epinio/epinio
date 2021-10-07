package acceptance_test

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/deployments"
	"github.com/epinio/epinio/helpers"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/names"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps", func() {
	var (
		org     string
		appName string
	)
	dockerImageURL := "splatform/sample-app"

	BeforeEach(func() {
		org = catalog.NewOrgName()
		env.SetupAndTargetOrg(org)

		appName = catalog.NewAppName()
	})

	When("creating an application without a workload", func() {
		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("creates the app", func() {
			out, err := env.Epinio("", "app", "create", appName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Ok"))
		})

		Context("with service", func() {
			var serviceName string

			BeforeEach(func() {
				serviceName = catalog.NewServiceName()
				env.MakeService(serviceName)
			})

			AfterEach(func() {
				env.DeleteServiceUnbind(serviceName)
				// env.DeleteApp see outer context
			})

			It("creates the app with instance count and services", func() {
				out, err := env.Epinio("", "app", "create", appName, "--bind", serviceName, "--instances", "2")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp("Ok"))

				out, err = env.Epinio("", "app", "show", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp(`Instances\s*\|\s*2\s*\|`))
				Expect(out).To(MatchRegexp(`Services\s*\|\s*` + serviceName + `\s*\|`))
			})
		})

		When("pushing a workload", func() {
			BeforeEach(func() {
				out, err := env.Epinio("", "app", "create", appName)
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("creates the workload", func() {
				appDir := "../assets/sample-app"
				out, err := env.Epinio(appDir, "app", "push", appName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring("App is online"))
			})
		})
	})

	When("pushing an app from an external repository", func() {
		It("pushes the app successfully", func() {
			wordpress := "https://github.com/epinio/example-wordpress"
			pushLog, err := env.Epinio("", "apps", "push", appName, wordpress, "--git", "main")
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
			It("respects the desired number of instances", func() {
				wordpress := "https://github.com/epinio/example-wordpress"
				pushLog, err := env.Epinio("", "apps", "push", appName, wordpress, "--git", "main")
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

				out, err := env.Epinio("", "app", "update", appName, "-i", "3")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

					return out
				}, "1m").Should(MatchRegexp(`Status\s*\|\s*3\/3\s*\|`))
			})

			AfterEach(func() {
				env.DeleteApp(appName)
			})
		})
	})

	When("pushing with custom builder flag", func() {
		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("uses the custom builder to stage", func() {
			By("Pushing a golang app")
			appDir := "../assets/golang-sample-app"
			pushLog, err := env.Epinio(appDir, "apps", "push", appName, "--builder-image", "paketobuildpacks/builder:tiny")
			Expect(err).ToNot(HaveOccurred(), pushLog)

			By("checking if the staging is using custom builder image")
			labels := fmt.Sprintf("app.kubernetes.io/name=%s,tekton.dev/pipelineTask=stage", appName)
			imageList, err := helpers.Kubectl("get", "pod",
				"--namespace", deployments.TektonStagingNamespace,
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

		act := func(arg ...string) (string, error) {
			appDir := "../assets/sample-app"
			return env.Epinio(appDir, "app", append([]string{"push", appName}, arg...)...)
		}

		replicas := func(ns, name string) string {
			n, err := helpers.Kubectl("get", "deployment",
				"--namespace", ns, name,
				"-o", "jsonpath={.status.replicas}")
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
				out, err := act()
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					return replicas(org, appName)
				}, timeout, interval).Should(Equal(strconv.Itoa(int(v1.DefaultInstances))))
			})
			By("pushing with an instance count", func() {
				out, err := act("--instances", "2")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					return replicas(org, appName)
				}, timeout, interval).Should(Equal("2"))
			})
			By("pushing again, without an instance count", func() {
				out, err := act()
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					return replicas(org, appName)
				}, timeout, interval).Should(Equal("2"))
			})
		})
	})

	Describe("build cache", func() {
		push := func(arg ...string) (string, error) {
			appDir := "../assets/sample-app"
			return env.Epinio(appDir, "app", append([]string{"push", appName}, arg...)...)
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
				out, err := helpers.Kubectl("get", "pvc", "-n",
					deployments.TektonStagingNamespace, names.GenerateResourceName(org, appName))
				Expect(err).ToNot(HaveOccurred(), out)

				out, err = push()
				Expect(err).ToNot(HaveOccurred(), out)

				Expect(out).To(MatchRegexp("Reusing cache layer"))
			})
		})
		When("deleting the app", func() {
			It("deletes the cache PVC too", func() {
				out, err := helpers.Kubectl("get", "pvc", "-n",
					deployments.TektonStagingNamespace, names.GenerateResourceName(org, appName))
				Expect(err).ToNot(HaveOccurred(), out)
				env.DeleteApp(appName)

				out, err = helpers.Kubectl("get", "pvc", "-n",
					deployments.TektonStagingNamespace, names.GenerateResourceName(org, appName))
				Expect(err).To(HaveOccurred(), out)
				Expect(out).To(MatchRegexp(fmt.Sprintf(`persistentvolumeclaims "%s" not found`, names.GenerateResourceName(org, appName))))
			})
		})
	})

	Describe("push and delete", func() {
		It("shows the staging logs", func() {
			By("pushing the app")
			out := env.MakeApp(appName, 1, true)

			Expect(out).To(MatchRegexp(`.*step-create.*Configuring PHP Application.*`))
			Expect(out).To(MatchRegexp(`.*step-create.*Using feature -- PHP.*`))
			// Doesn't include linkerd sidecar logs
			Expect(out).ToNot(MatchRegexp(`linkerd-.*`))
		})

		It("deploys a golang app", func() {
			out := env.MakeGolangApp(appName, 1, true)

			By("checking for the application resource", func() {
				Eventually(func() string {
					out, _ := helpers.Kubectl("get", "app",
						"--namespace", org, appName)
					return out
				}, "1m").Should(ContainSubstring("AGE")) // this checks for the table header from kubectl
			})

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
					out, _ := helpers.Kubectl("get", "app",
						"--namespace", org, appName)
					return out
				}, "1m").Should(ContainSubstring("NotFound"))
			})
		})

		It("deploys an app from the current dir", func() {
			By("pushing the app in the current working directory")
			out := env.MakeApp(appName, 1, true)

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

		It("removes the app's ingress when deleting an app", func() {
			env.MakeDockerImageApp(appName, 1, dockerImageURL)

			By("deleting the app")
			env.DeleteApp(appName)

			Eventually(func() string {
				out, _ := helpers.Kubectl("get", "ingress",
					"--namespace", org, appName)
				return out
			}, "1m").Should(ContainSubstring("not found"))

			Eventually(func() string {
				out, _ := helpers.Kubectl("get", "service",
					"--namespace", org, appName)
				return out
			}, "1m").Should(ContainSubstring("not found"))
		})

		It("should not fail for a max-length application name", func() {
			appNameLong := "app123456789012345678901234567890123456789012345678901234567890"
			// 3+60 characters
			env.MakeDockerImageApp(appNameLong, 1, dockerImageURL)

			By("deleting the app")
			env.DeleteApp(appNameLong)
		})

		It("should not fail for an application name with leading digits", func() {
			appNameLeadNumeric := "12monkeys"
			env.MakeDockerImageApp(appNameLeadNumeric, 1, dockerImageURL)

			By("deleting the app")
			env.DeleteApp(appNameLeadNumeric)
		})

		It("respects the desired number of instances", func() {
			app := catalog.NewAppName()
			env.MakeDockerImageApp(app, 3, dockerImageURL)
			defer env.DeleteApp(app)

			Eventually(func() string {
				out, err := env.Epinio("", "app", "show", app)
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").Should(MatchRegexp(`Status\s*\|\s*3\/3\s*\|`))
		})

		Context("with service", func() {
			var serviceName string

			BeforeEach(func() {
				serviceName = catalog.NewServiceName()
				env.MakeService(serviceName)
			})

			AfterEach(func() {
				env.DeleteApp(appName)
				env.DeleteService(serviceName)
			})

			It("pushes an app with bound services", func() {
				currentDir, err := os.Getwd()
				Expect(err).ToNot(HaveOccurred())

				pushOutput, err := env.Epinio(path.Join(currentDir, "../assets/sample-app"),
					"apps", "push", appName, "-b", serviceName)
				Expect(err).ToNot(HaveOccurred(), pushOutput)

				// And check presence
				Eventually(func() string {
					out, err := env.Epinio("", "app", "list")
					Expect(err).ToNot(HaveOccurred(), out)
					return out
				}, "2m").Should(MatchRegexp(appName + `.*\|.*1\/1.*\|.*` + serviceName))
			})
		})

		It("unbinds bound services when deleting an app, and then deletes the service", func() {
			serviceName := catalog.NewServiceName()

			env.MakeDockerImageApp(appName, 1, dockerImageURL)
			env.MakeService(serviceName)
			env.BindAppService(appName, serviceName, org)

			By("deleting the app")
			out, err := env.Epinio("", "app", "delete", appName)
			Expect(err).ToNot(HaveOccurred(), out)
			// TODO: Fix `epinio delete` from returning before the app is deleted #131

			Expect(out).To(MatchRegexp("UNBOUND SERVICES"))
			Expect(out).To(MatchRegexp(serviceName))

			Eventually(func() string {
				out, err := env.Epinio("", "app", "list")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").ShouldNot(MatchRegexp(`.*%s.*`, appName))

			env.DeleteService(serviceName)
		})
	})

	Describe("update", func() {
		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("respects the desired number of instances", func() {
			env.MakeDockerImageApp(appName, 1, dockerImageURL)

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

		Context("with service", func() {
			var serviceName string

			BeforeEach(func() {
				serviceName = catalog.NewServiceName()
				env.MakeService(serviceName)
			})

			AfterEach(func() {
				env.UnbindAppService(appName, serviceName, org)
				env.DeleteService(serviceName)
				// DeleteApp see outer context
			})

			It("respects the bound services", func() {
				env.MakeDockerImageApp(appName, 1, dockerImageURL)

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

					return out
				}, "1m").Should(MatchRegexp(`Status\s*\|\s*1\/1\s*\|`))

				out, err := env.Epinio("", "app", "update", appName, "--bind", serviceName)
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(MatchRegexp("Successfully updated application"))

				Eventually(func() string {
					out, err := env.Epinio("", "app", "show", appName)
					ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

					return out
				}, "1m").Should(MatchRegexp(`Services\s*\|\s*` + serviceName + `\s*\|`))
			})
		})
	})

	Describe("list and show", func() {
		var serviceName string
		BeforeEach(func() {
			serviceName = catalog.NewServiceName()
			env.MakeDockerImageApp(appName, 1, dockerImageURL)
			env.MakeService(serviceName)
			env.BindAppService(appName, serviceName, org)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
			env.CleanupService(serviceName)
		})

		It("lists all apps in the namespace", func() {
			out, err := env.Epinio("", "app", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Listing applications"))
			Expect(out).To(MatchRegexp(" " + appName + " "))
			Expect(out).To(MatchRegexp(" " + serviceName + " "))
		})

		It("shows the details of an app", func() {
			out, err := env.Epinio("", "app", "show", appName)
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Show application details"))
			Expect(out).To(MatchRegexp("Application: " + appName))
			Expect(out).To(MatchRegexp(`Services .*\|.* ` + serviceName))
			Expect(out).To(MatchRegexp(`Routes .*\|.* ` + appName))

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
		var org1 string
		var org2 string
		var app1 string
		var app2 string

		BeforeEach(func() {
			org1 = catalog.NewOrgName()
			env.SetupAndTargetOrg(org1)

			app1 = catalog.NewAppName()
			env.MakeDockerImageApp(app1, 1, dockerImageURL)

			org2 = catalog.NewOrgName()
			env.SetupAndTargetOrg(org2)

			app2 = catalog.NewAppName()
			env.MakeDockerImageApp(app2, 1, dockerImageURL)
		})

		AfterEach(func() {
			env.TargetOrg(org2)
			env.DeleteApp(app2)

			env.TargetOrg(org1)
			env.DeleteApp(app1)
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

			podNames := env.GetPodNames(appName, org)
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

			Expect(out).To(MatchRegexp(`.*step-create.*Configuring PHP Application.*`))
			Expect(out).To(MatchRegexp(`.*step-create.*Using feature -- PHP.*`))
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
})
