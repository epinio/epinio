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

// This test uses AWS route53 to update the system domain's records
var _ = Describe("<Scenario4>", func() {
	var (
		flags        []string
		epinioHelper epinio.Epinio
		appName      = catalog.NewAppName()
		loadbalancer string
		domain       string
		zoneID       string
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		domain = os.Getenv("AWS_DOMAIN")
		Expect(domain).ToNot(BeEmpty())

		zoneID = os.Getenv("AWS_ZONE_ID")
		Expect(zoneID).ToNot(BeEmpty())

		flags = []string{
			"--skip-default-namespace",
			"--system-domain=" + domain,
		}
	})

	AfterEach(func() {
		out, err := epinioHelper.Uninstall()
		Expect(err).NotTo(HaveOccurred(), out)
	})

	It("installs with loadbalancer IP, custom domain and pushes an app with env vars", func() {
		By("Installing Traefik", func() {
			out, err := epinioHelper.Run("install-ingress")
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(out).To(Or(ContainSubstring("Traefik deployed"), ContainSubstring("Traefik Ingress info")))
		})

		By("Extracting Loadbalancer Name", func() {
			out, err := proc.RunW("kubectl", "get", "service", "-n", "traefik", "traefik", "-o", "json")
			Expect(err).NotTo(HaveOccurred(), out)

			status := &testenv.LoadBalancerHostname{}
			err = json.Unmarshal([]byte(out), status)
			Expect(err).NotTo(HaveOccurred())
			Expect(status.Status.LoadBalancer.Ingress).To(HaveLen(1))
			loadbalancer = status.Status.LoadBalancer.Ingress[0].Hostname
			Expect(loadbalancer).ToNot(BeEmpty())
		})

		By("Updating DNS Entries", func() {
			change := route53.CNAME(domain, loadbalancer)
			out, err := route53.Upsert(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)

			change = route53.CNAME("*."+domain, loadbalancer)
			out, err = route53.Upsert(zoneID, change, nodeTmpDir)
			Expect(err).NotTo(HaveOccurred(), out)
		})

		// Check that DNS entry is correctly propagated
		By("Checking that DNS entry is correctly propagated", func() {
			Eventually(func() string {
				out, err := route53.TestDnsAnswer(zoneID, domain, "CNAME")
				Expect(err).NotTo(HaveOccurred(), out)

				answer := &route53.DNSAnswer{}
				err = json.Unmarshal([]byte(out), answer)
				Expect(err).NotTo(HaveOccurred())
				if len(answer.RecordData) == 0 {
					return ""
				}
				return answer.RecordData[0]
			}, "5m", "2s").Should(Equal(loadbalancer + ".")) // CNAME ends with a '.'
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

		By("Pushing an app with Env vars", func() {
			out, err := epinioHelper.Run("apps", "create", appName)
			Expect(err).NotTo(HaveOccurred(), out)

			out, err = epinioHelper.Run("apps", "env", "set", appName, "MYVAR", "myvalue")
			Expect(err).ToNot(HaveOccurred(), out)

			out, err = epinioHelper.Run("push",
				"--name", appName,
				"--path", testenv.AssetPath("sample-app"))
			Expect(err).ToNot(HaveOccurred(), out)

			Eventually(func() string {
				out, err := proc.RunW("kubectl", "get", "deployment", "--namespace", testenv.DefaultWorkspace, appName, "-o", "jsonpath={.spec.template.spec.containers[0].env}")
				Expect(err).ToNot(HaveOccurred(), out)
				return out
			}).Should(MatchRegexp("MYVAR"))
		})
	})
})
