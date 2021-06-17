package acceptance_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("Show", func() {
		It("shows the configuration", func() {
			config, err := Epinio("config show", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(MatchRegexp(`Current Organization.*\|.*workspace`))
			Expect(config).To(MatchRegexp(`Certificates.*\|.*Present`))
			Expect(config).To(MatchRegexp(fmt.Sprintf(`API User Name.*\|.*%s`, epinioUser)))
			Expect(config).To(MatchRegexp(fmt.Sprintf(`API Password.*\|.*%s`, epinioPassword)))
		})
	})

	Describe("Update-Credentials", func() {
		BeforeEach(func() {
			// Set current configuration aside
			out, err := RunProc(fmt.Sprintf("mv %s/epinio.yaml %s/epinio.yaml.bak", nodeTmpDir, nodeTmpDir), "", false)
			Expect(err).ToNot(HaveOccurred(), out)
		})

		AfterEach(func() {
			// Restore full configuration
			out, err := RunProc(fmt.Sprintf("mv %s/epinio.yaml.bak %s/epinio.yaml", nodeTmpDir, nodeTmpDir), "", false)
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("regenerates certs and credentials", func() {
			// Get back the certs and credentials
			// Note that org, as a purely local setting, is not restored
			_, err := Epinio("config update-credentials", "")
			Expect(err).ToNot(HaveOccurred())

			newConfig, err := getConfig()
			Expect(err).ToNot(HaveOccurred())

			oldConfig, err := getConfigFrom(fmt.Sprintf("%s/epinio.yaml.bak", nodeTmpDir))
			Expect(err).ToNot(HaveOccurred())

			Expect(newConfig.Certs).To(Equal(oldConfig.Certs))
			Expect(newConfig.Password).To(Equal(oldConfig.Password))
			Expect(newConfig.Certs).To(Equal(oldConfig.Certs))
		})
	})
})
