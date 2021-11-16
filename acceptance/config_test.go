package acceptance_test

import (
	"fmt"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var tmpConfigPath string

	BeforeEach(func() {
		tmpConfigPath = catalog.NewTmpName("tmpEpinio") + `.yaml`
	})

	AfterEach(func() {
		// Remove transient config
		out, err := proc.Run("", false, "rm", "-f", tmpConfigPath)
		Expect(err).ToNot(HaveOccurred(), out)
	})

	Describe("Ensemble", func() {
		It("fails for a bogus sub command", func() {
			out, err := env.Epinio("", "config", "bogus", "...")
			Expect(err).To(HaveOccurred())
			Expect(out).To(MatchRegexp(`Unknown method "bogus"`))
		})
	})

	Describe("Colors", func() {
		It("changes the configuration when disabling colors", func() {
			config, err := env.Epinio("", "config", "colors", "0", "--config-file", tmpConfigPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(MatchRegexp(`Colors: false`))

			config, err = env.Epinio("", "config", "show", "--config-file", tmpConfigPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(MatchRegexp(`Colorized Output.*\|.*false`))
		})

		It("changes the configuration when enabling colors", func() {
			config, err := env.Epinio("", "config", "colors", "1", "--config-file", tmpConfigPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(MatchRegexp(`Colors: true`))

			config, err = env.Epinio("", "config", "show", "--config-file", tmpConfigPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(MatchRegexp(`Colorized Output.*\|.*true`))
		})
	})

	Describe("Show", func() {
		It("shows the configuration", func() {
			config, err := env.Epinio("", "config", "show")
			Expect(err).ToNot(HaveOccurred())
			Expect(config).To(MatchRegexp(`Colorized Output.*\|`))  // Exact state not relevant
			Expect(config).To(MatchRegexp(`Current Namespace.*\|`)) // Exact name of namespace is not relevant, and varies
			Expect(config).To(MatchRegexp(`Certificates.*\|.*Present`))
			Expect(config).To(MatchRegexp(fmt.Sprintf(`API User Name.*\|.*%s`, env.EpinioUser)))
			Expect(config).To(MatchRegexp(fmt.Sprintf(`API Password.*\|.*%s`, env.EpinioPassword)))
			Expect(config).To(MatchRegexp(`API Url.*\| https://epinio.*`))
			Expect(config).To(MatchRegexp(`WSS Url.*\| wss://epinio.*`))
		})
	})

	Describe("Update", func() {
		oldConfigPath := testenv.EpinioYAML()

		It("regenerates certs and credentials", func() {
			// Get back the certs and credentials
			// Note that `namespace`, as a purely local setting, is not restored

			out, err := env.Epinio("", "config", "update", "--config-file", tmpConfigPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(MatchRegexp(`Updating the stored credentials`))

			oldConfig, err := env.GetConfigFrom(oldConfigPath)
			Expect(err).ToNot(HaveOccurred())

			newConfig, err := env.GetConfigFrom(tmpConfigPath)
			Expect(err).ToNot(HaveOccurred())

			Expect(newConfig.User).To(Equal(oldConfig.User))
			Expect(newConfig.Password).To(Equal(oldConfig.Password))
			Expect(newConfig.API).To(Equal(oldConfig.API))
			Expect(newConfig.WSS).To(Equal(oldConfig.WSS))
			Expect(newConfig.Certs).To(Equal(oldConfig.Certs))
		})
	})
})
