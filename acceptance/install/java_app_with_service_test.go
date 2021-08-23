package install_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
)

// | Name            | Client Platform       | Server Platform             | Install Test                   | Install Options           | Push Test | Push Options        |
// | --------------- | --------------------- | --------------------------- | ------------------------------ | ------------------------- | --------- | ------------------- |
// |                 | _Actions Runner_      | _Cluster Provider_          | _Operator_                     |                           | _Developer_                     |
// | JavaAppWithService | Windows            | Azure                       | Split Install for Cert Manager | Private CA                | Java App  | Create with Service |

var _ = Describe("Install with private CA and push <JavaAppWithService>", func() {
	var (
		flags        []string
		epinioHelper epinio.Epinio
		appDir       string
		serviceName  string
		// appName      string
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		flags = []string{
			"--skip-default-org",
			"--skip-cert-manager",
			"--tls-issuer=private-ca",
		}

		var err error
		appDir, err = ioutil.TempDir("", "epinio-app")
		Expect(err).NotTo(HaveOccurred())

		app := testenv.DownloadDescriptor{
			URL:         "https://github.com/spring-projects/spring-petclinic/archive/refs/heads/main.tar.gz",
			Compression: testenv.TarGZ,
			Destination: appDir,
		}
		err = testenv.Download(app)
		Expect(err).NotTo(HaveOccurred())

		serviceName = catalog.NewServiceName()
		// appName = catalog.NewAppName()
	})

	AfterEach(func() {
		os.RemoveAll(appDir)

		out, err := epinioHelper.Uninstall()
		Expect(err).NotTo(HaveOccurred(), out)
	})

	It("installs and passes scenario", func() {
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

		By("Creating a customo service and pushing an app", func() {
			//testenv.SetupInClusterServices(testenv.EpinioBinaryPath())
			out, err := epinioHelper.Run("service", "create-custom", serviceName, "mariadb", "10-3-22")
			Expect(err).NotTo(HaveOccurred(), out)
			// TODO fix timeout, maybe use a smaller java app
			// out, err = epinioHelper.Run("push", appName, appDir, "--bind", serviceName)
			// Expect(err).NotTo(HaveOccurred(), out)
		})
	})
})
