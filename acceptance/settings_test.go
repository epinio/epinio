package acceptance_test

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Settings", func() {
	var tmpSettingsPath string

	BeforeEach(func() {
		tmpSettingsPath = catalog.NewTmpName("tmpEpinio") + `.yaml`
	})

	AfterEach(func() {
		// Remove transient settings
		out, err := proc.Run("", false, "rm", "-f", tmpSettingsPath)
		Expect(err).ToNot(HaveOccurred(), out)
	})

	Describe("Ensemble", func() {
		It("fails for a bogus sub command", func() {
			out, err := env.Epinio("", "settings", "bogus", "...")
			Expect(err).To(HaveOccurred())
			Expect(out).To(ContainSubstring(`Unknown method "bogus"`))
		})
	})

	Describe("Colors", func() {
		It("changes the settings when disabling colors", func() {
			settings, err := env.Epinio("", "settings", "colors", "0", "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(settings).To(ContainSubstring(`Colors: false`))

			settings, err = env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(settings).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Colorized Output", "false"),
				),
			)
		})

		It("changes the settings when enabling colors", func() {
			settings, err := env.Epinio("", "settings", "colors", "1", "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(settings).To(ContainSubstring(`Colors: true`))

			settings, err = env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(settings).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Colorized Output", "true"),
				),
			)
		})
	})

	Describe("Show", func() {
		It("shows the settings", func() {
			settings, err := env.Epinio("", "settings", "show")
			Expect(err).ToNot(HaveOccurred())
			Expect(settings).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Colorized Output", "true|false"),
					WithRow("Current Namespace", ".*"),
					WithRow("Default App Chart", ""),
					WithRow("API User Name", env.EpinioToken),
					WithRow("API Password", "[*]+"),
					WithRow("API Url", "https://epinio.*"),
					WithRow("WSS Url", "wss://epinio.*"),
					WithRow("Certificates", "Present"),
				),
			)
		})

		It("shows empty settings", func() {
			settings, err := env.Epinio("", "settings", "show", "--settings-file", "/tmp/empty")
			Expect(err).ToNot(HaveOccurred())
			Expect(settings).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Colorized Output", "true|false"),
					WithRow("Current Namespace", ".*"),
					WithRow("Default App Chart", ""),
					WithRow("API User Name", ""),
					WithRow("API Password", ""),
					WithRow("API Url", ""),
					WithRow("WSS Url", ""),
					WithRow("Certificates", "None defined"),
				),
			)
		})

		It("shows the settings with the password in plaintext", func() {
			settings, err := env.Epinio("", "settings", "show", "--show-token")
			Expect(err).ToNot(HaveOccurred())
			Expect(settings).To(
				HaveATable(
					WithHeaders("KEY", "VALUE"),
					WithRow("Colorized Output", "true|false"),
					WithRow("Current Namespace", ".*"),
					WithRow("Certificates", "Present"),
					WithRow("API Token", env.EpinioToken),
					WithRow("API Url", "https://epinio.*"),
					WithRow("WSS Url", "wss://epinio.*"),
				),
			)
		})
	})

	Describe("UpdateCA", func() {
		oldSettingsPath := testenv.EpinioYAML()

		It("regenerates certs and credentials", func() {
			// Get back the certs and credentials
			// Note that `namespace`, as a purely local setting, is not restored

			out, err := env.Epinio("", "settings", "update-ca", "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring(`Updating CA in the stored credentials`))

			oldSettings, err := env.GetSettingsFrom(oldSettingsPath)
			Expect(err).ToNot(HaveOccurred())

			newSettings, err := env.GetSettingsFrom(tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred())

			Expect(newSettings.API).To(Equal(oldSettings.API))
			Expect(newSettings.WSS).To(Equal(oldSettings.WSS))
			Expect(newSettings.Certs).To(Equal(oldSettings.Certs))
		})

		It("stores the password in base64", func() {
			out, err := env.Epinio("", "settings", "update-ca", "--settings-file", tmpSettingsPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring(`Updating CA in the stored credentials`))

			settings, err := env.GetSettingsFrom(oldSettingsPath)
			Expect(err).ToNot(HaveOccurred())

			fileContents, err := ioutil.ReadFile(oldSettingsPath)
			Expect(err).ToNot(HaveOccurred())
			encodedPass := base64.StdEncoding.EncodeToString([]byte(settings.Token.AccessToken))
			Expect(string(fileContents)).To(MatchRegexp(fmt.Sprintf("pass: %s", encodedPass)))
		})
	})
})
