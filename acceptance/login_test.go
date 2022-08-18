package acceptance_test

import (
	"bytes"
	"os/exec"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Login", func() {
	var tmpSettingsPath string

	BeforeEach(func() {
		tmpSettingsPath = catalog.NewTmpName("tmpEpinio") + `.yaml`
	})

	AfterEach(func() {
		// Remove transient settings
		out, err := proc.Run("", false, "rm", "-f", tmpSettingsPath)
		Expect(err).ToNot(HaveOccurred(), out)
	})

	It("succeeds with a valid user", func() {
		// check that the initial settings are empty
		settings, err := env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
		Expect(err).ToNot(HaveOccurred(), settings)
		Expect(settings).To(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("API User Name", ""),
				WithRow("API Password", ""),
				WithRow("Certificates", "None defined"),
			),
		)

		// login with a different user
		out, err := env.Epinio("", "login", "-u", "epinio", "-p", env.EpinioToken,
			"--trust-ca", "--settings-file", tmpSettingsPath, serverURL)
		Expect(err).ToNot(HaveOccurred())
		Expect(out).To(ContainSubstring(`Login to your Epinio cluster`))
		Expect(out).To(ContainSubstring(`Trusting certificate...`))
		Expect(out).To(ContainSubstring(`Login successful`))

		// check that the settings are now updated
		settings, err = env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
		Expect(err).ToNot(HaveOccurred(), settings)
		Expect(settings).To(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("API User Name", "epinio"),
				WithRow("API Password", "[*]+"),
				WithRow("Certificates", "Present"),
			),
		)
	})

	It("succeeds with an interactively entered valid user [fixed bug]", func() {
		// Note: The user is entered with leading and trailing whitespace
		// Note: Which the system has to ignore.

		// Attention!! While it is desired to have this kind of test for the
		// password as well a naive implementation does not work. It results in
		// error `inappropriate ioctl for device`.
		//
		// The underlying issue is that the user name is read by a simple
		// `NewReader`. Whereas the password is read via `term.ReadPassword`,
		// i.e. directly from the underlying TTY. Which is a good thing, i.e. more
		// secure, no echoing of the input, etc.
		//
		// Still, it also prevents us from using a simple redirect of the stdin to
		// a bytes buffer, as that has no proper TTY.
		//
		// Result, for now no password test.

		// check that the initial settings are empty
		settings, err := env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
		Expect(err).ToNot(HaveOccurred(), settings)
		Expect(settings).To(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("API User Name", ""),
				WithRow("API Password", ""),
				WithRow("Certificates", "None defined"),
			),
		)

		// login with a different user - name is specified interactively on stdin

		var out bytes.Buffer
		cmd := exec.Command(testenv.EpinioBinaryPath(), "login", "-p", env.EpinioToken,
			"--trust-ca", "--settings-file", tmpSettingsPath, serverURL)
		cmd.Stdin = bytes.NewReader([]byte("  epinio    \r\n"))
		cmd.Stdout = &out
		cmd.Stderr = &out

		err = cmd.Run()

		Expect(err).ToNot(HaveOccurred())
		Expect(out.String()).To(ContainSubstring(`Login to your Epinio cluster`))
		Expect(out.String()).To(ContainSubstring(`Trusting certificate...`))
		Expect(out.String()).To(ContainSubstring(`Login successful`))

		// check that the settings are now updated
		settings, err = env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
		Expect(err).ToNot(HaveOccurred(), settings)
		Expect(settings).To(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("API User Name", "epinio"),
				WithRow("API Password", "[*]+"),
				WithRow("Certificates", "Present"),
			),
		)
	})

	It("fails with a non existing user", func() {
		// check that the initial settings are empty
		settings, err := env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
		Expect(err).ToNot(HaveOccurred(), settings)
		Expect(settings).To(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("API User Name", ""),
				WithRow("API Password", ""),
				WithRow("Certificates", "None defined"),
			),
		)

		// login with a non existing user
		out, err := env.Epinio("", "login", "-u", "unknown", "-p", env.EpinioToken,
			"--trust-ca", "--settings-file", tmpSettingsPath, serverURL)
		Expect(err).To(HaveOccurred(), out)
		Expect(out).To(ContainSubstring(`error verifying credentials`))

		// check that the initial settings are still empty
		settings, err = env.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
		Expect(err).ToNot(HaveOccurred(), settings)
		Expect(settings).To(
			HaveATable(
				WithHeaders("KEY", "VALUE"),
				WithRow("API User Name", ""),
				WithRow("API Password", ""),
				WithRow("Certificates", "None defined"),
			),
		)
	})

	// It("respects the port when one is present [fixed bug]", func() {
	// 	randomPort := fmt.Sprintf(`:%d`, rand.Intn(65536))
	// 	serverURLWithPort := serverURL + randomPort

	// 	out, err := env.Epinio("", "login", "-u", "epinio", "-p", env.EpinioPassword,
	// 		"--trust-ca", "--settings-file", tmpSettingsPath, serverURLWithPort)
	// 	Expect(err).To(HaveOccurred(), out)

	// 	// split and filter the lines to check that the port is present in both of them
	// 	outLines := []string{}
	// 	for _, l := range strings.Split(out, "\n") {
	// 		if strings.TrimSpace(l) != "" {
	// 			outLines = append(outLines, l)
	// 		}
	// 	}

	// 	Expect(outLines[0]).To(ContainSubstring(`Login to your Epinio cluster`))
	// 	Expect(outLines[0]).To(ContainSubstring(randomPort))

	// 	Expect(outLines[1]).To(ContainSubstring(`error while checking CA`))
	// 	Expect(outLines[1]).To(ContainSubstring(randomPort + `: connect: connection refused`))
	// })
})
