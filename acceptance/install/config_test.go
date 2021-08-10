package acceptance_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"

	"github.com/epinio/epinio/acceptance/helpers/epinio"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Epinio Installation", func() {
	var (
		configFile string
	)

	epinioBinary := fmt.Sprintf("../../dist/epinio-%s-%s", runtime.GOOS, runtime.GOARCH)
	epinioHelper := epinio.NewEpinioHelper(epinioBinary)
	epinioUser := "epinio"
	epinioPassword := "password"

	BeforeEach(func() {
		input, err := ioutil.ReadFile("../../assets/tests/config.yaml")
		Expect(err).NotTo(HaveOccurred())
		f, err := ioutil.TempFile("", "config")
		Expect(err).NotTo(HaveOccurred())

		configFile = f.Name()
		err = ioutil.WriteFile(configFile, input, 0644)
		Expect(err).NotTo(HaveOccurred())
		err = f.Close()
		Expect(err).NotTo(HaveOccurred())

		epinioHelper.Flags = []string{
			"--config-file", configFile,
			"--skip-default-org",
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
			out, err := epinioHelper.Install()
			Expect(err).NotTo(HaveOccurred())

			By("Checking for updated values in epinio config file")
			data, err := ioutil.ReadFile(configFile)
			Expect(err).NotTo(HaveOccurred())
			dataString := string(data)

			// The values for checking are taken from ./assets/tests/config.yaml
			Expect(dataString).NotTo(ContainSubstring("pass: 05ec82a894940780"))
			Expect(dataString).NotTo(ContainSubstring("user: 996ee615fde2ceed"))
			Expect(dataString).To(ContainSubstring("pass: password"))
			Expect(dataString).To(ContainSubstring("user: epinio"))

			By("Checking the values in the stdout")
			Expect(out).To(ContainSubstring("API Password: password"))
			Expect(out).To(ContainSubstring("API User: epinio"))
		})
	})
})
