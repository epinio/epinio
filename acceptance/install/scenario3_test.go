package install_test

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
)

var _ = Describe("<Scenario3> RKE, Private CA, Service", func() {
	var (
		flags        []string
		epinioHelper epinio.Epinio
		appName      = catalog.NewAppName()
		serviceName  = catalog.NewServiceName()
		loadbalancer string
		metallbURL   string
		localpathURL string
		// testenv.New is not needed for VerifyAppServiceBound helper :shrug:
		env      testenv.EpinioEnv
		domainIP = "192.168.1.240" // Set it to an arbitrary private IP
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		metallbURL = "https://raw.githubusercontent.com/google/metallb/v0.10.3/manifests/metallb.yaml"
		localpathURL = "https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.20/deploy/local-path-storage.yaml"

		flags = []string{
			"--system-domain", fmt.Sprintf("%s.omg.howdoi.website", domainIP),
			"--skip-cert-manager",
			"--tls-issuer=private-ca",
		}

	})

	AfterEach(func() {
		out, err := epinioHelper.Uninstall()
		Expect(err).NotTo(HaveOccurred(), out)
	})

	It("installs with private CA and pushes an app with service", func() {
		By("Installing MetalLB", func() {
			out, err := proc.RunW("kubectl", "create", "namespace", "metallb-system")
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = proc.RunW("kubectl", "apply", "-f", metallbURL)
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = proc.RunW("sed", "-i", fmt.Sprintf("s/myip/%s/g", domainIP), testenv.TestAssetPath("config-metallb-rke.yml"))
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = proc.RunW("kubectl", "apply", "-f", testenv.TestAssetPath("config-metallb-rke.yml"))
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
			out, err := epinioHelper.Run("install-cert-manager")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("CertManager deployed"))

			// Create certificate secret and cluster_issuer
			out, err = proc.RunW("kubectl", "apply", "-f", testenv.TestAssetPath("cluster-issuer-private-ca.yml"))
			Expect(err).NotTo(HaveOccurred(), out)
		})

		By("Installing Epinio", func() {
			out, err := epinioHelper.Install(flags...)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("Epinio installed."))

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
