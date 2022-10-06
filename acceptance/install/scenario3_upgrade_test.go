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
	"github.com/epinio/epinio/acceptance/testenv"
)

var _ = Describe("<Scenario3 Up> RKE, Private CA, Configuration, on External Registry", func() {
	var (
		flags             []string
		epinioHelper      epinio.Epinio
		configurationName = catalog.NewConfigurationName()
		appName           string
		loadbalancer      string
		registryUsername  string
		registryPassword  string
		rangeIP           string
		domain            string
		domainIP          string
		extraEnvName      string
		extraEnvValue     string
		name_exists       bool
		value_exists      bool
		localpathURL      = "https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.20/deploy/local-path-storage.yaml"
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		// Clean previous installed helm repos
		// Done at the beginning because we don't know the runner's state
		out, err := proc.Run(testenv.Root(), false, "bash", "./scripts/remove-helm-repos.sh")
		Expect(err).NotTo(HaveOccurred(), out)

		domain = os.Getenv("EPINIO_SYSTEM_DOMAIN")
		Expect(domain).ToNot(BeEmpty())
		domainIP = strings.TrimSuffix(domain, ".omg.howdoi.website")

		appName = "external-reg-test-rke"

		registryUsername = os.Getenv("REGISTRY_USERNAME")
		Expect(registryUsername).ToNot(BeEmpty())

		registryPassword = os.Getenv("REGISTRY_PASSWORD")
		Expect(registryPassword).ToNot(BeEmpty())
		flags = []string{
			"--set", "global.domain=" + domain,
			"--set", "global.tlsIssuer=private-ca",
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

	It("Installs with private CA, pushes an app with configuration, and upgrades", func() {
		By("Installing MetalLB", func() {
			rangeIP = os.Getenv("RANGE_IP")
			out, err := proc.RunW("sed", "-i", fmt.Sprintf("s/@IP_RANGE@/%s/", rangeIP),
				testenv.TestAssetPath("resources.yaml"))
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = proc.RunW("helm", "repo", "add", "metallb", "https://metallb.github.io/metallb")
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = proc.RunW("helm", "upgrade", "--install", "--wait", "-n", "metallb",
				"--create-namespace", "metallb", "metallb/metallb")
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = proc.RunW("kubectl", "apply", "-f", testenv.TestAssetPath("resources.yaml"))
			Expect(err).NotTo(HaveOccurred(), out)
		})

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

			// We need to be sure that the specified IP is really used
			Expect(loadbalancer).To(Equal(domainIP))
		})

		By("Configuring local-path storage", func() {
			out, err := proc.RunW("kubectl", "apply", "-f", localpathURL)
			Expect(err).NotTo(HaveOccurred(), out)

			value := `{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}`
			out, err = proc.RunW("kubectl", "patch", "storageclass", "local-path", "-p", value)
			Expect(err).NotTo(HaveOccurred(), out)
		})

		By("Creating private CA issuer", func() {
			// Create certificate secret and cluster_issuer
			out, err := proc.RunW("kubectl", "apply", "-f", testenv.TestAssetPath("cluster-issuer-private-ca.yml"))
			Expect(err).NotTo(HaveOccurred(), out)
		})

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

		By("Creating a configuration and pushing an app", func() {
			out, err := epinioHelper.Run("configuration", "create", configurationName, "mariadb", "10-3-22")
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = epinioHelper.Run("push",
				"--name", appName,
				"--path", testenv.AssetPath("sample-app"),
				"--bind", configurationName)
			Expect(err).NotTo(HaveOccurred(), out)

			env.VerifyAppConfigurationBound(appName, configurationName, testenv.DefaultWorkspace, 1)

			// Verify cluster_issuer is used
			out, err = proc.RunW("kubectl", "get", "certificate",
				"-n", testenv.DefaultWorkspace,
				"--selector", "app.kubernetes.io/name="+appName,
				"-o", "jsonpath='{.items[*].spec.issuerRef.name}'")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal("'private-ca'"))
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
				catentry  string // Service catalog entry to use
			)

			By("Setup And Checks Before Upgrade", func() {
				catentry = "mysql-dev"
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

				// Deploy a simple application before upgrading Epinio, check that it is reachable
				By("Deploy application pre-upgrade")
				env.MakeGolangApp(beforeApp, 1, true)
				beforeRoute = fmt.Sprintf("https://%s.%s", beforeApp, domain)
				By("Route: " + beforeRoute)
				Eventually(func() int {
					resp, err := env.Curl("GET", beforeRoute, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					return resp.StatusCode
				}, 30*time.Second, 1*time.Second).Should(Equal(http.StatusOK))

				// Check that we can create a configuration before the upgrade
				By("Create configuration pre-upgrade")
				out, err := env.Epinio("", "configuration", "create", beforeConfig, "fox", "lair")
				Expect(err).ToNot(HaveOccurred(), out)

				// Check that we can create a service before the upgrade
				By("Create service pre-upgrade")
				env.MakeServiceInstance(beforeService, catentry)

				// Check that a custom catalog entry is visible
				By("Create custom catalog entry pre-upgrade")
				catalog.CreateCatalogService(catalog.NginxCatalogService(beforeCatalog))
				out, err = env.Epinio("", "service", "catalog")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(beforeCatalog))
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
				By("Checking configuration existence ...")
				env.HaveConfiguration(beforeConfig)

				// Check that the before service instance still exists
				// -- TODO -- full incompatibility -- pre/post spike
				//By("Checking service existence ...")
				//env.HaveServiceInstance(beforeService)

				By("Create configuration post-upgrade")
				out, err := env.Epinio("", "configuration", "create", afterConfig, "dog", "house")
				Expect(err).ToNot(HaveOccurred(), out)

				By("Create service post-upgrade")
				env.MakeServiceInstance(afterService, catentry)

				By("Create custom catalog entry post-upgrade")
				catalog.CreateCatalogService(catalog.NginxCatalogService(afterCatalog))
				out, err = env.Epinio("", "service", "catalog")
				Expect(err).ToNot(HaveOccurred(), out)
				Expect(out).To(ContainSubstring(afterCatalog))

				// Check that we can create an application after the upgrade, incl. reachability
				By("Create application post-upgrade")
				env.MakeGolangApp(afterApp, 1, true)
				afterRoute = fmt.Sprintf("https://%s.%s", afterApp, domain)
				By("Route: " + afterRoute)
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
				// env.DeleteService("nginxB-" + afterService)
				// env.DeleteConfiguration(afterConfig)
				// env.DeleteConfiguration(beforeConfig)
				catalog.DeleteCatalogService(afterCatalog)
				catalog.DeleteCatalogService(beforeCatalog)
				env.DeleteNamespace(namespace)
			})
		})
	})
})
