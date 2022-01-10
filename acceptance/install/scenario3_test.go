package install_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
)

var _ = Describe("<Scenario3> RKE, Private CA, Service, on External Registry", func() {
	var (
		flags            []string
		epinioHelper     epinio.Epinio
		serviceName      = catalog.NewServiceName()
		appName          string
		loadbalancer     string
		registryUsername string
		registryPassword string
		rangeIP          string
		domainIP         []string
		// testenv.New is not needed for VerifyAppServiceBound helper :shrug:
		env          testenv.EpinioEnv
		localpathURL = "https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.20/deploy/local-path-storage.yaml"
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		// Clean previous installed helm repos
		// Done at the beginning because we don't know the runner's state
		out, err := proc.Run(testenv.Root(), false, "bash", "./scripts/remove-helm-repos.sh")
		Expect(err).NotTo(HaveOccurred(), out)

		// Get a free IP address on server's network
		rangeIP, _ = proc.Run(testenv.Root(), false, "bash", "./scripts/get-free-ip.sh")
		domainIP = strings.Split(rangeIP, "-")

		appName = "externalregtest"

		registryUsername = os.Getenv("REGISTRY_USERNAME")
		Expect(registryUsername).ToNot(BeEmpty())

		registryPassword = os.Getenv("REGISTRY_PASSWORD")
		Expect(registryPassword).ToNot(BeEmpty())
		flags = []string{
			"--set", "skipCertManager=true",
			"--set", "domain=" + fmt.Sprintf("%s.omg.howdoi.website", domainIP[0]),
			"--set", "tlsIssuer=private-ca",
			"--set", "externalRegistryURL=registry.hub.docker.com",
			"--set", "externalRegistryUsername=" + registryUsername,
			"--set", "externalRegistryPassword=" + registryPassword,
			"--set", "externalRegistryNamespace=splatform",
		}

	})

	AfterEach(func() {
		out, err := epinioHelper.Uninstall()
		Expect(err).NotTo(HaveOccurred(), out)

		out, err = proc.RunW("helm", "uninstall", "-n", "metallb", "metallb")
		Expect(err).NotTo(HaveOccurred(), out)
	})

	It("installs with private CA and pushes an app with service", func() {
		By("Installing MetalLB", func() {
			out, err := proc.RunW("sed", "-i", fmt.Sprintf("s/@IP_RANGE@/%s/", rangeIP),
				testenv.TestAssetPath("values-metallb-rke.yml"))
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = proc.RunW("helm", "repo", "add", "metallb", "https://metallb.github.io/metallb")
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = proc.RunW("helm", "upgrade", "--install", "-n", "metallb",
				"--create-namespace", "metallb", "metallb/metallb", "-f",
				testenv.TestAssetPath("values-metallb-rke.yml"))
			Expect(err).NotTo(HaveOccurred(), out)
		})

		By("Configuring local-path storage", func() {
			out, err := proc.RunW("kubectl", "apply", "-f", localpathURL)
			Expect(err).NotTo(HaveOccurred(), out)

			value := `{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}`
			out, err = proc.RunW("kubectl", "patch", "storageclass", "local-path", "-p", value)
			Expect(err).NotTo(HaveOccurred(), out)
		})

		By("Installing CertManager", func() {
			out, err := proc.RunW("helm", "repo", "add", "jetstack", "https://charts.jetstack.io")
			Expect(err).NotTo(HaveOccurred(), out)
			out, err = proc.RunW("helm", "repo", "update")
			Expect(err).NotTo(HaveOccurred(), out)
			out, err = proc.RunW("helm", "upgrade", "--install", "cert-manager", "jetstack/cert-manager",
				"-n", "cert-manager",
				"--create-namespace",
				"--set", "installCRDs=true",
				"--set", "extraArgs[0]=--enable-certificate-owner-ref=true",
			)
			Expect(err).NotTo(HaveOccurred(), out)

			// Create certificate secret and cluster_issuer
			out, err = proc.RunW("kubectl", "apply", "-f", testenv.TestAssetPath("cluster-issuer-private-ca.yml"))
			Expect(err).NotTo(HaveOccurred(), out)
		})

		By("Installing Epinio", func() {
			out, err := epinioHelper.Install(flags...)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("STATUS: deployed"))

			out, err = testenv.PatchEpinio()
			Expect(err).ToNot(HaveOccurred(), out)
		})

		By("Checking Loadbalancer IP", func() {
			out, err := proc.RunW("kubectl", "get", "service", "-n", "traefik", "traefik", "-o", "json")
			Expect(err).NotTo(HaveOccurred(), out)

			status := &testenv.LoadBalancerHostname{}
			err = json.Unmarshal([]byte(out), status)
			Expect(err).NotTo(HaveOccurred())
			Expect(status.Status.LoadBalancer.Ingress).To(HaveLen(1))
			loadbalancer = status.Status.LoadBalancer.Ingress[0].IP
			Expect(loadbalancer).ToNot(BeEmpty())
			// We need to be sure that the specified IP is really used
			Expect(loadbalancer).To(Equal(domainIP[0]))
		})

		By("Updating Epinio config", func() {
			out, err := epinioHelper.Run("config", "update")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Ok"))
		})

		By("Checking Epinio info command", func() {
			Eventually(func() string {
				out, _ := epinioHelper.Run("info")
				return out
			}, "2m", "2s").Should(ContainSubstring("Epinio Version:"))
		})

		By("Creating a service and pushing an app", func() {
			out, err := epinioHelper.Run("service", "create", serviceName, "mariadb", "10-3-22")
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = epinioHelper.Run("push",
				"--name", appName,
				"--path", testenv.AssetPath("sample-app"),
				"--bind", serviceName)
			Expect(err).NotTo(HaveOccurred(), out)

			env.VerifyAppServiceBound(appName, serviceName, testenv.DefaultWorkspace, 1)

			// Verify cluster_issuer is used
			out, err = proc.RunW("kubectl", "get", "certificate",
				"-n", testenv.DefaultWorkspace,
				"--selector", "app.kubernetes.io/name="+appName,
				"-o", "jsonpath='{.items[*].spec.issuerRef.name}'")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal("'private-ca'"))
		})
	})
})
