package install_test

import (
	"io/ioutil"
	"os"

	"github.com/epinio/epinio/acceptance/helpers/epinio"
	"github.com/epinio/epinio/acceptance/testenv"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// | Name            | Client Platform       | Server Platform             | Install Test                   | Install Options           | Push Test | Push Options        |
// | --------------- | --------------------- | --------------------------- | ------------------------------ | ------------------------- | --------- | ------------------- |
// |                 | _Actions Runner_      | _Cluster Provider_          | _Operator_                     |                           | _Developer_                     |
// | ConfigFile      | Linux                 | K3d                         | Configfile Flag                | Magic DNS                 | PHP App   | Create, then push   |

var _ = Describe("Install with <ConfigFile> and push a PHP app", func() {
	var (
		configFile     string
		epinioUser     = "epinio"
		epinioPassword = "password"
		flags          []string
		epinioHelper   epinio.Epinio
	)

	BeforeEach(func() {
		epinioHelper = epinio.NewEpinioHelper(testenv.EpinioBinaryPath())

		input, err := ioutil.ReadFile(testenv.TestAssetPath("config.yaml"))
		Expect(err).NotTo(HaveOccurred())
		f, err := ioutil.TempFile("", "config")
		Expect(err).NotTo(HaveOccurred())

		configFile = f.Name()
		err = ioutil.WriteFile(configFile, input, 0644)
		Expect(err).NotTo(HaveOccurred())
		err = f.Close()
		Expect(err).NotTo(HaveOccurred())

		flags = []string{
			"--config-file", configFile,
			"--skip-default-namespace",
			"--user", epinioUser,
			"--password", epinioPassword,
		}
	})

	AfterEach(func() {
		epinioHelper.Uninstall()

		err := os.Remove(configFile)
		Expect(err).NotTo(HaveOccurred())
	})

	When("a epinio config file already exists", func() {
		It("should install epinio with new values and update the file", func() {
			By("Installing epinio")
			out, err := epinioHelper.Install(flags...)
			Expect(err).NotTo(HaveOccurred(), out)

			By("Checking the values in the stdout")
			Expect(out).To(ContainSubstring("API Password: password"))
			Expect(out).To(ContainSubstring("API User: epinio"))

			By("Checking for updated values in epinio config file")
			data, err := ioutil.ReadFile(configFile)
			Expect(err).NotTo(HaveOccurred())
			dataString := string(data)

			// The values for checking are taken from ./assets/tests/config.yaml
			Expect(dataString).NotTo(ContainSubstring("pass: 05ec82a894940780"))
			Expect(dataString).NotTo(ContainSubstring("user: 996ee615fde2ceed"))
			Expect(dataString).To(ContainSubstring("pass: password"))
			Expect(dataString).To(ContainSubstring("user: epinio"))
		})
	})
})
