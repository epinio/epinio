package install_test

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/helpers/route53"
	"github.com/epinio/epinio/acceptance/testenv"
)

var _ = Describe("<Scenario3> Azure, Private CA, Service", func() {
	var (
		appName      = catalog.NewAppName()
		domain       string
		epinioHelper epinio.Epinio
		flags        []string
		loadbalancer string
		serviceName  = catalog.NewServiceName()
		zoneID       string
		// testenv.New is not needed for VerifyAppServiceBound helper :shrug:
		env testenv.EpinioEnv
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		// use Route53
		domain = os.Getenv("AKS_DOMAIN")
		Expect(domain).ToNot(BeEmpty())

		zoneID = os.Getenv("AWS_ZONE_ID")
		Expect(zoneID).ToNot(BeEmpty())

		flags = []string{
			"--skip-default-namespace",
			"--skip-cert-manager",
			"--tls-issuer=private-ca",
			"--system-domain=" + domain,
		}
	})

	AfterEach(func() {
		out, err := epinioHelper.Uninstall()
		Expect(err).NotTo(HaveOccurred(), out)
	})

	It("installs and passes scenario", func() {
		By("Installing Traefik", func() {
			out, err := epinioHelper.Run("install-ingress")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Or(ContainSubstring("Traefik deployed"), ContainSubstring("Traefik Ingress info")))
		})

		By("Extracting AKS Loadbalancer Name", func() {
			out, err := proc.RunW("kubectl", "get", "service", "-n", "traefik", "traefik", "-o", "json")
			Expect(err).NotTo(HaveOccurred(), out)

			status := &testenv.LoadBalancerHostname{}
			err = json.Unmarshal([]byte(out), status)
			Expect(err).NotTo(HaveOccurred())
			Expect(status.Status.LoadBalancer.Ingress).To(HaveLen(1))
			loadbalancer = status.Status.LoadBalancer.Ingress[0].IP
			Expect(loadbalancer).ToNot(BeEmpty(), out)
		})

		By("Updating DNS Entries", func() {
			change := route53.A(domain, loadbalancer)
			out, err := route53.Upsert(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)

			change = route53.A("*."+domain, loadbalancer)
			out, err = route53.Upsert(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)
		})

		By("Installing CertManager", func() {
			out, err := epinioHelper.Run("install-cert-manager")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("CertManager deployed"))

			// create certificate secret and cluster_issuer
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

		// Now create the default org which we skipped because
		// it would fail before patching.
		testenv.EnsureDefaultWorkspace(testenv.EpinioBinaryPath())
		out, err := epinioHelper.Run("target", testenv.DefaultWorkspace)
		Expect(err).ToNot(HaveOccurred(), out)

		By("Creating a service and pushing an app", func() {
			out, err := epinioHelper.Run("service", "create", serviceName, "mariadb", "10-3-22")
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = epinioHelper.Run("push",
				"--name", appName,
				"--path", testenv.AssetPath("sample-app"),
				"--bind", serviceName)
			Expect(err).NotTo(HaveOccurred(), out)

			env.VerifyAppServiceBound(appName, serviceName, testenv.DefaultWorkspace, 1)

			// verify cluster_issuer is used
			out, err = proc.RunW("kubectl", "get", "certificate",
				"-n", testenv.DefaultWorkspace, appName, "-o", "jsonpath='{.spec.issuerRef.name}'")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Equal("'private-ca'"))
		})
	})
})
