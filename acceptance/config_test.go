package acceptance_test

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/proc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	Describe("Colors", func() {
		It("changes the configuration when disabling colors", func() {
			config, err := env.Epinio("config colors 0", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(MatchRegexp(`Colors: false`))

			config, err = env.Epinio("config show", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(MatchRegexp(`Colorized Output.*\|.*false`))
		})

		It("changes the configuration when enabling colors", func() {
			config, err := env.Epinio("config colors 1", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(MatchRegexp(`Colors: true`))

			config, err = env.Epinio("config show", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(MatchRegexp(`Colorized Output.*\|.*true`))
		})
	})

	Describe("Show", func() {
		It("shows the configuration", func() {
			config, err := env.Epinio("config show", "")
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(MatchRegexp(`Colorized Output.*\|`))     // Exact state not relevant
			Expect(config).To(MatchRegexp(`Current Organization.*\|`)) // Exact name of org is not relevant, and varies
			Expect(config).To(MatchRegexp(`Certificates.*\|.*Present`))
			Expect(config).To(MatchRegexp(fmt.Sprintf(`API User Name.*\|.*%s`, env.EpinioUser)))
			Expect(config).To(MatchRegexp(fmt.Sprintf(`API Password.*\|.*%s`, env.EpinioPassword)))
		})
	})

	Describe("Update-Credentials", func() {
		BeforeEach(func() {
			// Set current configuration aside
			out, err := proc.Run(fmt.Sprintf("mv %s/epinio.yaml %s/epinio.yaml.bak", nodeTmpDir, nodeTmpDir), "", false)
			Expect(err).ToNot(HaveOccurred(), out)
		})

		AfterEach(func() {
			// Restore full configuration
			out, err := proc.Run(fmt.Sprintf("mv %s/epinio.yaml.bak %s/epinio.yaml", nodeTmpDir, nodeTmpDir), "", false)
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("regenerates certs and credentials", func() {
			// Get back the certs and credentials
			// Note that org, as a purely local setting, is not restored
			_, err := env.Epinio("config update-credentials", "")
			Expect(err).ToNot(HaveOccurred())

			newConfig, err := env.GetConfig()
			Expect(err).ToNot(HaveOccurred())

			oldConfig, err := env.GetConfigFrom(fmt.Sprintf("%s/epinio.yaml.bak", nodeTmpDir))
			Expect(err).ToNot(HaveOccurred())

			Expect(newConfig.Certs).To(Equal(oldConfig.Certs))
			Expect(newConfig.Password).To(Equal(oldConfig.Password))
			Expect(newConfig.Certs).To(Equal(oldConfig.Certs))
		})
	})
})
