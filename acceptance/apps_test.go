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

	"github.com/epinio/epinio/helpers"
	v1 "github.com/epinio/epinio/internal/api/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps", func() {
	var (
		org     string
		appName string
	)

	BeforeEach(func() {
		org = newOrgName()
		setupAndTargetOrg(org)

		appName = newAppName()
	})

	When("pushing an app multiple times", func() {
		var (
			timeout  = 30 * time.Second
			interval = 1 * time.Second
		)

		act := func(arg string) (string, error) {
			appDir := "../assets/sample-app"
			return Epinio(fmt.Sprintf("apps push %[1]s %[2]s", appName, arg), appDir)
		}

		replicas := func(ns, name string) string {
			n, err := helpers.Kubectl(fmt.Sprintf("get deployment --namespace %s %s -o=jsonpath='{.status.replicas}'", ns, name))
			if err != nil {
				return ""
			}
			return n
		}

		It("pushes the same app again successfully", func() {
			makeApp(appName, 1, false)

			By("pushing the app again")
			makeApp(appName, 1, false)

			By("deleting the app")
			deleteApp(appName)
		})

		It("honours the given instance count", func() {
			By("pushing without instance count", func() {
				out, err := act("")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					return replicas(org, appName)
				}, timeout, interval).Should(Equal(strconv.Itoa(int(v1.DefaultInstances))))
			})
			By("pushing with an instance count", func() {
				out, err := act("--instances 2")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					return replicas(org, appName)
				}, timeout, interval).Should(Equal("2"))
			})
			By("pushing again, without an instance count", func() {
				out, err := act("")
				Expect(err).ToNot(HaveOccurred(), out)

				Eventually(func() string {
					return replicas(org, appName)
				}, timeout, interval).Should(Equal("2"))
			})
		})
	})

	Describe("push and delete", func() {
		It("shows the staging logs", func() {
			By("pushing the app")
			out := makeApp(appName, 1, true)

			Expect(out).To(MatchRegexp(`.*step-create.*Configuring PHP Application.*`))
			Expect(out).To(MatchRegexp(`.*step-create.*Using feature -- PHP.*`))
			// Doesn't include linkerd sidecar logs
			Expect(out).ToNot(MatchRegexp(`linkerd-.*`))
		})

		It("deploys a golang app", func() {
			out := makeGolangApp(appName, 1, true)

			By("checking for the application resource", func() {
				Eventually(func() string {
					out, _ := helpers.Kubectl(fmt.Sprintf("get app --namespace %s %s",
						org, appName))
					return out
				}, "1m").Should(ContainSubstring("AGE")) // this checks for the table header from kubectl
			})

			routeRegexp := regexp.MustCompile(`https:\/\/.*omg.howdoi.website`)
			route := string(routeRegexp.Find([]byte(out)))

			Eventually(func() int {
				resp, err := Curl("GET", route, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				return resp.StatusCode
			}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

			By("deleting the app")
			deleteApp(appName)

			By("checking the application resource was removed", func() {
				Eventually(func() string {
					out, _ := helpers.Kubectl(fmt.Sprintf("get app --namespace %s %s",
						org, appName))
					return out
				}, "1m").Should(ContainSubstring("NotFound"))
			})
		})

		It("deploys an app from the current dir", func() {
			By("pushing the app in the current working directory")
			out := makeApp(appName, 1, true)

			routeRegexp := regexp.MustCompile(`https:\/\/.*omg.howdoi.website`)
			route := string(routeRegexp.Find([]byte(out)))

			Eventually(func() int {
				resp, err := Curl("GET", route, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				return resp.StatusCode
			}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

			By("deleting the app")
			deleteApp(appName)
		})

		It("deploys an app from the specified dir", func() {
			By("pushing the app in the specified app directory")
			makeApp(appName, 1, false)

			By("deleting the app")
			deleteApp(appName)
		})

		It("removes the app's ingress when deleting an app", func() {
			makeApp(appName, 1, false)

			By("deleting the app")
			deleteApp(appName)

			Eventually(func() string {
				out, _ := helpers.Kubectl(fmt.Sprintf("get ingress --namespace %s %s",
					org, appName))
				return out
			}, "1m").Should(ContainSubstring("not found"))

			Eventually(func() string {
				out, _ := helpers.Kubectl(fmt.Sprintf("get service --namespace %s %s",
					org, appName))
				return out
			}, "1m").Should(ContainSubstring("not found"))
		})

		It("should not fail for a max-length application name", func() {
			appNameLong := "app123456789012345678901234567890123456789012345678901234567890"
			// 3+60 characters
			makeApp(appNameLong, 1, false)

			By("deleting the app")
			deleteApp(appNameLong)
		})

		It("respects the desired number of instances", func() {
			app := newAppName()
			makeApp(app, 3, true)
			defer deleteApp(app)

			Eventually(func() string {
				out, err := Epinio("app show "+app, "")
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").Should(MatchRegexp(`Status\s*\|\s*3\/3\s*\|`))
		})

		Context("with service", func() {
			var serviceName string

			BeforeEach(func() {
				serviceName = newServiceName()
				makeCustomService(serviceName)
			})

			AfterEach(func() {
				deleteApp(appName)
				deleteService(serviceName)
			})

			It("pushes an app with bound services", func() {
				currentDir, err := os.Getwd()
				Expect(err).ToNot(HaveOccurred())

				pushOutput, err := Epinio(fmt.Sprintf("apps push %s -b %s",
					appName, serviceName),
					path.Join(currentDir, "../assets/sample-app"))
				Expect(err).ToNot(HaveOccurred(), pushOutput)

				// And check presence
				Eventually(func() string {
					out, err := Epinio("app list", "")
					Expect(err).ToNot(HaveOccurred(), out)
					return out
				}, "2m").Should(MatchRegexp(appName + `.*\|.*1\/1.*\|.*` + serviceName))
			})
		})

		It("unbinds bound services when deleting an app", func() {
			serviceName := newServiceName()

			makeApp(appName, 1, true)
			makeCustomService(serviceName)
			bindAppService(appName, serviceName, org)

			By("deleting the app")
			out, err := Epinio("app delete "+appName, "")
			Expect(err).ToNot(HaveOccurred(), out)
			// TODO: Fix `epinio delete` from returning before the app is deleted #131

			Expect(out).To(MatchRegexp("UNBOUND SERVICES"))
			Expect(out).To(MatchRegexp(serviceName))

			Eventually(func() string {
				out, err := Epinio("app list", "")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").ShouldNot(MatchRegexp(`.*%s.*`, appName))
		})
	})

	Describe("update", func() {
		It("respects the desired number of instances", func() {
			makeApp(appName, 1, true)
			defer deleteApp(appName)

			Eventually(func() string {
				out, err := Epinio("app show "+appName, "")
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").Should(MatchRegexp(`Status\s*\|\s*1\/1\s*\|`))

			out, err := Epinio(fmt.Sprintf("app update %s -i 3", appName), "")
			Expect(err).ToNot(HaveOccurred(), out)

			Eventually(func() string {
				out, err := Epinio("app show "+appName, "")
				ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

				return out
			}, "1m").Should(MatchRegexp(`Status\s*\|\s*3\/3\s*\|`))
		})
	})

	Describe("list and show", func() {
		var serviceCustomName string
		BeforeEach(func() {
			serviceCustomName = newServiceName()
			makeApp(appName, 1, true)
			makeCustomService(serviceCustomName)
			bindAppService(appName, serviceCustomName, org)
		})

		AfterEach(func() {
			deleteApp(appName)
			cleanupService(serviceCustomName)
		})

		It("lists all apps", func() {
			out, err := Epinio("app list", "")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Listing applications"))
			Expect(out).To(MatchRegexp(" " + appName + " "))
			Expect(out).To(MatchRegexp(" " + serviceCustomName + " "))
		})

		It("shows the details of an app", func() {
			out, err := Epinio("app show "+appName, "")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp("Show application details"))
			Expect(out).To(MatchRegexp("Application: " + appName))
			Expect(out).To(MatchRegexp(`Services .*\|.* ` + serviceCustomName))
			Expect(out).To(MatchRegexp(`Routes .*\|.* ` + appName))

			Eventually(func() string {
				out, err = Epinio("app show "+appName, "")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}, "1m").Should(MatchRegexp(`Status .*\|.* 1\/1`))
		})
	})

	Describe("logs", func() {
		var (
			route     string
			logLength int
		)

		BeforeEach(func() {
			out := makeApp(appName, 1, true)
			routeRegexp := regexp.MustCompile(`https:\/\/.*omg.howdoi.website`)
			route = string(routeRegexp.Find([]byte(out)))

			out, err := Epinio("app logs "+appName, "")
			Expect(err).ToNot(HaveOccurred(), out)

			podNames := getPodNames(appName, org)
			for _, podName := range podNames {
				Expect(out).To(ContainSubstring(podName))
			}
			logs := strings.Split(out, "\n")
			logLength = len(logs)
		})

		AfterEach(func() {
			deleteApp(appName)
		})

		It("shows the staging logs", func() {
			out, err := Epinio("app logs --staging "+appName, "")
			Expect(err).ToNot(HaveOccurred(), out)

			Expect(out).To(MatchRegexp(`.*step-create.*Configuring PHP Application.*`))
			Expect(out).To(MatchRegexp(`.*step-create.*Using feature -- PHP.*`))
		})

		It("follows logs", func() {
			p, err := GetProc(nodeTmpDir+"/epinio app logs --follow "+appName, "")
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
				resp, err := Curl("GET", route, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				return resp.StatusCode
			}, "1m").Should(Equal(http.StatusOK))

			By("checking the latest log")
			scanner.Scan()
			Expect(scanner.Text()).To(ContainSubstring("GET / HTTP/1.1"))
		})
	})
})
