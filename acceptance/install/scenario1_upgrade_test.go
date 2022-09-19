package install_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/helpers/route53"
	"github.com/epinio/epinio/acceptance/testenv"
)

// This test uses AWS route53 to update the system domain's records
var _ = Describe("<Scenario1 Up> GKE, epinio-ca", func() {
	var (
		flags         []string
		epinioHelper  epinio.Epinio
		appName       = catalog.NewAppName()
		loadbalancer  string
		domain        string
		zoneID        string
		extraEnvName  string
		extraEnvValue string
		name_exists   bool
		value_exists  bool
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		domain = os.Getenv("EPINIO_SYSTEM_DOMAIN")
		By("Domain: " + domain)
		Expect(domain).ToNot(BeEmpty())

		zoneID = os.Getenv("AWS_ZONE_ID")
		By("Zone:   " + zoneID)
		Expect(zoneID).ToNot(BeEmpty())

		flags = []string{
			"--set", "global.domain=" + domain,
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
				// after*  - after upgrade
				// before* - before upgrade

				afterApp     string // Application
				afterCatalog string // Service Catalog
				afterConfig  string // Configuration
				afterRoute   string // App route
				afterService string // Service

				beforeApp     string // Same as above, before upgrade
				beforeCatalog string
				beforeConfig  string
				beforeRoute   string
				beforeService string

				namespace string // Namespace (created before upgrade)

				// catentry string // Service catalog entry to use
			)

			By("Setup And Checks Before Upgrade", func() {
				// catentry = "redis-dev"

				namespace = catalog.NewNamespaceName()
				env.SetupAndTargetNamespace(namespace)
				beforeApp = catalog.NewAppName()
				afterApp = catalog.NewAppName()
				afterService = catalog.NewServiceNamePrefixed("after")
				beforeService = catalog.NewServiceNamePrefixed("before")
				afterConfig = catalog.NewConfigurationName()
				beforeConfig = catalog.NewConfigurationName()
				beforeCatalog = catalog.NewCatalogServiceNamePrefixed("before")
				afterCatalog = catalog.NewCatalogServiceNamePrefixed("after")

				// Note current versions of client and server
				By("Versions before upgrade")
				env.Versions()

				// Deploy a simple application before upgrading Epinio
				env.MakeGolangApp(beforeApp, 1, true)
				beforeRoute = fmt.Sprintf("https://%s.%s", beforeApp, domain)
				By("Route: " + beforeRoute)

				// Check that the app is reachable
				Eventually(func() int {
					resp, err := env.Curl("GET", beforeRoute, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					return resp.StatusCode
				}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

				// Check that we can create a configuration before the upgrade
				By("Creating a configuration pre-upgrade")

				out, err := env.Epinio("", "configuration", "create", beforeConfig, "fox", "lair")
				Expect(err).ToNot(HaveOccurred(), out)

				// Check that we can create a service before the upgrade
				By("Creating a service pre-upgrade")
				// env.MakeServiceInstance(beforeService, catentry)

				catalog.CreateCatalogService(catalog.NginxCatalogService(beforeCatalog))

				out, err = env.Epinio("", "service", "catalog")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(beforeCatalog))

				env.MakeServiceInstance("nginxB-"+beforeService, beforeCatalog)

				// By("----------------------------------------------- ZZZ CRD BEFORE")
				// env.ListCRDS()
				// env.SeeCRD("services.application.epinio.io")
				// env.ListServiceCRS()
				// env.SeeServiceCR(catentry)
				// By("----------------------------------------------- ZZZ")
			})

			By("Upgrading actual", func() {
				// Upgrade to current as found in checkout
				epinioHelper.Upgrade()
			})

			By("Checks After Upgrade", func() {
				// Note post-upgrade versions of client and server
				By("Versions after upgrade")
				env.Versions()

				// Check that the before app is still reachable
				By("Checking reachability ...")
				Eventually(func() int {
					resp, err := env.Curl("GET", beforeRoute, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					return resp.StatusCode
				}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

				// Check that the before configuration still exists
				env.HaveConfiguration(beforeConfig)

				// Check that the before service instance still exists
				// env.HaveServiceInstance(beforeService)

				// --------------------------------------------------------------------------

				// Check that we can create a configuration after the upgrade
				By("Creating a configuration post-upgrade")

				out, err := env.Epinio("", "configuration", "create", afterConfig, "dog", "house")
				Expect(err).ToNot(HaveOccurred(), out)

				catalog.CreateCatalogService(catalog.NginxCatalogService(afterCatalog))

				out, err = env.Epinio("", "service", "catalog")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(afterCatalog))

				// env.MakeServiceInstance("nginxA-"+afterService, afterCatalog)
				env.MakeServiceInstance("nginxB-"+afterService, beforeCatalog)

				// Check that we can create a service after the upgrade
				// -- disabled -- By("Creating a service post-upgrade")
				//
				// By("----------------------------------------------- ZZZ CRD AFTER")
				// env.ListCRDS()
				// env.SeeCRD("services.application.epinio.io")
				// env.ListServiceCRS()
				// env.SeeServiceCR(catentry)
				// By("----------------------------------------------- ZZZ")

				// env.MakeServiceInstance(afterService, catentry)

				// Check that we can create an application after the upgrade
				By("Creating an application post-upgrade")

				env.MakeGolangApp(afterApp, 1, true)
				afterRoute = fmt.Sprintf("https://%s.%s", afterApp, domain)
				By("Route: " + afterRoute)

				// Check that the app is reachable
				Eventually(func() int {
					resp, err := env.Curl("GET", afterRoute, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					return resp.StatusCode
				}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))
			})

			By("Teardown After Upgrade", func() {
				env.DeleteApp(beforeApp)
				env.DeleteApp(afterApp)
				// env.DeleteService(afterService)
				// env.DeleteService(beforeService)
				// env.DeleteService("nginxA-" + afterService)
				env.DeleteService("nginxB-" + afterService)
				// env.DeleteConfiguration(afterConfig)
				// env.DeleteConfiguration(beforeConfig)
				env.DeleteNamespace(namespace)
				catalog.DeleteCatalogService(afterCatalog)
				catalog.DeleteCatalogService(beforeCatalog)
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
