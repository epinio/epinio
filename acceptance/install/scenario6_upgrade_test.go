package install_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/helpers/route53"
	"github.com/epinio/epinio/acceptance/testenv"
)

var _ = Describe("<Scenario6 Up> Azure, epinio-ca, External Registry", func() {
	var (
		appName          string
		domain           string
		epinioHelper     epinio.Epinio
		flags            []string
		loadbalancer     string
		registryUsername string
		registryPassword string
		zoneID           string
		extraEnvName     string
		extraEnvValue    string
		name_exists      bool
		value_exists     bool
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		// use Route53
		domain = os.Getenv("EPINIO_SYSTEM_DOMAIN")
		Expect(domain).ToNot(BeEmpty())

		zoneID = os.Getenv("AWS_ZONE_ID")
		Expect(zoneID).ToNot(BeEmpty())

		appName = "external-reg-test-aks"

		registryUsername = os.Getenv("REGISTRY_USERNAME")
		Expect(registryUsername).ToNot(BeEmpty())

		registryPassword = os.Getenv("REGISTRY_PASSWORD")
		Expect(registryPassword).ToNot(BeEmpty())
		flags = []string{
			"--set", "global.domain=" + domain,
			"--set", "containerregistry.enabled=false",
			"--set", "global.registryURL=registry.hub.docker.com",
			"--set", "global.registryUsername=" + registryUsername,
			"--set", "global.registryPassword=" + registryPassword,
			"--set", "global.registryNamespace=splatform",
		}

		extraEnvName, name_exists = os.LookupEnv("EXTRAENV_NAME")
		extraEnvValue, value_exists = os.LookupEnv("EXTRAENV_VALUE")
		if name_exists && value_exists {
			flags = append(flags, "--set", "extraEnv[0].name="+extraEnvName, "--set-string", "extraEnv[0].value="+extraEnvValue)
		}
	})

	AfterEach(func() {
		out, err := epinioHelper.Uninstall()
		Expect(err).NotTo(HaveOccurred(), out)
	})

	It("Installs with loadbalancer IP, custom domain, pushes an app, and upgrades", func() {
		By("Checking LoadBalancer IP", func() {
			// Ensure that Traefik LB is not in Pending state anymore, could take time
			Eventually(func() string {
				out, err := proc.RunW("kubectl", "get", "svc", "-n", "traefik", "traefik", "--no-headers")
				Expect(err).NotTo(HaveOccurred(), out)
				return out
			}, "4m", "2s").ShouldNot(ContainSubstring("<pending>"))

			out, err := proc.RunW("kubectl", "get", "service", "-n", "traefik", "traefik", "-o", "json")
			Expect(err).NotTo(HaveOccurred(), out)

			// Check that an IP address for LB is configured
			status := &testenv.LoadBalancerHostname{}
			err = json.Unmarshal([]byte(out), status)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(status.Status.LoadBalancer.Ingress).To(HaveLen(1))
			loadbalancer = status.Status.LoadBalancer.Ingress[0].IP
			Expect(loadbalancer).ToNot(BeEmpty())
		})

		By("Updating DNS Entries", func() {
			change := route53.A(domain, loadbalancer, "UPSERT")
			out, err := route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)

			change = route53.A("*."+domain, loadbalancer, "UPSERT")
			out, err = route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)
		})

		// Check that DNS entry is correctly propagated
		By("Checking that DNS entry is correctly propagated", func() {
			Eventually(func() string {
				out, err := route53.TestDnsAnswer(zoneID, domain, "A")
				Expect(err).NotTo(HaveOccurred(), out)

				answer := &route53.DNSAnswer{}
				err = json.Unmarshal([]byte(out), answer)
				Expect(err).NotTo(HaveOccurred(), out)
				if len(answer.RecordData) == 0 {
					return ""
				}
				return answer.RecordData[0]
			}, "5m", "2s").Should(Equal(loadbalancer))
		})

		// Workaround to (try to!) ensure that the DNS is really propagated!
		time.Sleep(3 * time.Minute)

		By("Installing Epinio", func() {
			out, err := epinioHelper.Install(flags...)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("STATUS: deployed"))

			out, err = testenv.PatchEpinio()
			Expect(err).ToNot(HaveOccurred(), out)
		})

		By("Connecting to Epinio", func() {
			Eventually(func() string {
				out, _ := epinioHelper.Run("login", "-u", "admin", "-p", "password", "--trust-ca", "https://epinio."+domain)
				return out
			}, "2m", "5s").Should(ContainSubstring("Login successful"))
		})

		By("Checking Epinio info command", func() {
			Eventually(func() string {
				out, _ := epinioHelper.Run("info")
				return out
			}, "2m", "2s").Should(ContainSubstring("Epinio Server Version:"))
		})

		By("Pushing an app", func() {
			out, err := epinioHelper.Run("push",
				"--name", appName,
				"--path", testenv.AssetPath("sample-app"))
			Expect(err).NotTo(HaveOccurred(), out)

			// Verify cluster_issuer is used
			out, err = proc.RunW("kubectl", "get", "certificate",
				"-n", testenv.DefaultWorkspace,
				"--selector", "app.kubernetes.io/name="+appName,
				"-o", "jsonpath='{.items[*].spec.issuerRef.name}'")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal("'epinio-ca'"))
		})

		By("Delete an app", func() {
			out, err := epinioHelper.Run("apps", "delete", appName)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Or(ContainSubstring("Application deleted")))
		})

		By("Upgrading", func() {
			var (
				// a* - after upgrade
				// b* - before upgrade

				namespace string // Namespace (created before upgrade)
				aapp      string // Application
				aconfig   string // Configuration
				aservice  string // Service
				bapp      string // Application
				bconfig   string // Configuration
				bservice  string // Service
				route     string // App routes
			)

			By("Setup And Checks Before Upgrade", func() {
				namespace = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespace)
				bapp = catalog.NewAppName()
				aapp = catalog.NewAppName()
				aservice = catalog.NewServiceName()
				bservice = catalog.NewServiceName()
				aconfig = catalog.NewConfigurationName()
				bconfig = catalog.NewConfigurationName()

				// Note current versions of client and server
				By("Versions before upgrade")
				env.Versions()

				// Deploy a simple application before upgrading Epinio
				env.MakeGolangApp(bapp, 1, true)
				route = fmt.Sprintf("https://%s.%s", bapp, domain)
				By("Route: " + route)

				// Check that the app is reachable
				Eventually(func() int {
					resp, err := env.Curl("GET", route, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					return resp.StatusCode
				}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

				// Check that we can create a configuration before the upgrade
				By("Creating a configuration pre-upgrade")

				out, err := env.Epinio("", "configuration", "create", bconfig, "fox", "lair")
				Expect(err).ToNot(HaveOccurred(), out)

				// Check that we can create a service before the upgrade
				By("Creating a service pre-upgrade")

				out, err = env.Epinio("", "service", "create", "mysql-dev", bservice)
				Expect(err).ToNot(HaveOccurred(), out)

				By("wait for deployment")
				Eventually(func() string {
					out, _ := env.Epinio("", "service", "show", aservice)
					return out
				}, "2m", "5s").Should(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Status", "deployed"),
					),
				)
			})

			By("Upgrading actual", func() {
				// Upgrade to current as found in checkout
				epinioHelper.Upgrade()
			})

			By("Checks After Upgrade", func() {
				// Note post-upgrade versions of client and server
				By("Versions after upgrade")
				env.Versions()

				// Check that the app is still reachable
				By("Checking reachability ...")
				Eventually(func() int {
					resp, err := env.Curl("GET", route, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					return resp.StatusCode
				}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

				// Check that we can create a configuration before the upgrade
				By("Creating a configuration pre-upgrade")

				out, err := env.Epinio("", "configuration", "create", aconfig, "dog", "house")
				Expect(err).ToNot(HaveOccurred(), out)

				// Check that we can create a service after the upgrade
				By("Creating a service post-upgrade")

				out, err = env.Epinio("", "service", "create", "mysql-dev", aservice)
				Expect(err).ToNot(HaveOccurred(), out)

				By("wait for deployment")
				Eventually(func() string {
					out, _ := env.Epinio("", "service", "show", aservice)
					return out
				}, "2m", "5s").Should(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Status", "deployed"),
					),
				)

				// Check that we can create an application after the upgrade
				By("Creating an application post-upgrade")

				env.MakeGolangApp(aapp, 1, true)
				route := fmt.Sprintf("https://%s.%s", aapp, domain)
				By("Route: " + route)

				// Check that the app is reachable
				Eventually(func() int {
					resp, err := env.Curl("GET", route, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					return resp.StatusCode
				}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))
			})

			By("Teardown After Upgrade", func() {
				env.DeleteApp(bapp)
				env.DeleteApp(aapp)
				env.DeleteService(aservice)
				env.DeleteService(bservice)
				// env.DeleteConfiguration(aconfig)
				// env.DeleteConfiguration(bconfig)
				env.DeleteNamespace(namespace)
			})
		})

		By("Cleaning DNS Entries", func() {
			change := route53.A(domain, loadbalancer, "DELETE")
			out, err := route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)

			change = route53.A("*."+domain, loadbalancer, "DELETE")
			out, err = route53.Update(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)
		})
	})
})
