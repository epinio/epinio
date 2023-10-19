// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package acceptance_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Login", LMisc, func() {
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
		// check that the initial settings are empt
		ExpectEmptySettings(tmpSettingsPath)

		// login with a valid user
		_ = ExpectGoodUserLogin(tmpSettingsPath, env.EpinioPassword, serverURL)
		// check that the settings are now updated
		ExpectUserPasswordSettings(tmpSettingsPath)
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
		// Result, for now, no password test.

		// check that the initial settings are empty
		ExpectEmptySettings(tmpSettingsPath)

		// login with a different user - name is specified interactively on stdin
		var out bytes.Buffer
		cmd := exec.Command(testenv.EpinioBinaryPath(), "login", "-p", env.EpinioPassword,
			"--trust-ca", "--settings-file", tmpSettingsPath, serverURL)
		cmd.Stdin = bytes.NewReader([]byte("  epinio    \r\n"))
		cmd.Stdout = &out
		cmd.Stderr = &out

		err := cmd.Run()

		Expect(err).ToNot(HaveOccurred())
		Expect(out.String()).To(ContainSubstring(`Login to your Epinio cluster`))
		Expect(out.String()).To(ContainSubstring(`Trusting certificate`))
		Expect(out.String()).To(ContainSubstring(`Login successful`))

		// check that the settings are now updated
		ExpectUserPasswordSettings(tmpSettingsPath)
	})

	It("succeeds with OIDC", func() {
		// check that the initial settings are empty
		ExpectEmptySettings(tmpSettingsPath)

		// login with valid token
		ExpectGoodTokenLogin(tmpSettingsPath, serverURL)
		// check that the settings are now updated
		ExpectTokenSettings(tmpSettingsPath)
	})

	It("performs implied logout of previous oidc login", func() {
		// check that the initial settings are empty
		ExpectEmptySettings(tmpSettingsPath)

		// login with valid token
		ExpectGoodTokenLogin(tmpSettingsPath, serverURL)
		// check that the settings are now updated
		ExpectTokenSettings(tmpSettingsPath)

		// login with a valid user
		_ = ExpectGoodUserLogin(tmpSettingsPath, env.EpinioPassword, serverURL)
		// check that the settings are now updated
		ExpectUserPasswordSettings(tmpSettingsPath)
	})

	It("performs implied logout of previous regular login", func() {
		// check that the initial settings are empty
		ExpectEmptySettings(tmpSettingsPath)

		// login with a valid user
		_ = ExpectGoodUserLogin(tmpSettingsPath, env.EpinioPassword, serverURL)
		// check that the settings are now updated
		ExpectUserPasswordSettings(tmpSettingsPath)

		// login with valid token
		ExpectGoodTokenLogin(tmpSettingsPath, serverURL)
		// check that the settings are now updated
		ExpectTokenSettings(tmpSettingsPath)
	})

	It("fails with a non existing user", func() {
		// check that the initial settings are empty
		ExpectEmptySettings(tmpSettingsPath)

		// login with a non existing user
		out, err := env.Epinio("", "login", "-u", "unknown", "-p", env.EpinioPassword,
			"--trust-ca", "--settings-file", tmpSettingsPath, serverURL)
		Expect(err).To(HaveOccurred(), out)
		Expect(out).To(ContainSubstring(`error verifying credentials`))

		// check that the settings are still empty
		ExpectEmptySettings(tmpSettingsPath)
	})

	It("clears a bogus current namespace", func() {
		// check that the initial settings are empty
		ExpectEmptySettings(tmpSettingsPath)

		// place a bogus namespace into the settings -- cannot be done with epinio, it reject such
		err := os.WriteFile(tmpSettingsPath, []byte(`namespace: bogus`), 0600)
		Expect(err).ToNot(HaveOccurred())
		ExpectNamespace(tmpSettingsPath, "bogus")

		// login with a valid user
		out := ExpectGoodUserLogin(tmpSettingsPath, env.EpinioPassword, serverURL)

		Expect(out).To(ContainSubstring("Current namespace 'bogus' not found in targeted cluster"))
		Expect(out).To(ContainSubstring("Cleared current namespace"))
		Expect(out).To(ContainSubstring("Please use `epinio target` to chose a new current namespace"))

		// check that current namespace is empty
		ExpectNamespace(tmpSettingsPath, "")
	})

	It("respects the port when one is present [fixed bug]", func() {
		randomPort := fmt.Sprintf(`:%d`, r.Intn(65536))
		serverURLWithPort := serverURL + randomPort

		out, err := env.Epinio("", "login", "-u", "epinio", "-p", env.EpinioPassword,
			"--trust-ca", "--settings-file", tmpSettingsPath, serverURLWithPort)
		Expect(err).To(HaveOccurred(), out)

		// split and filter the lines to check that the port is present in both of them
		outLines := []string{}
		for _, l := range strings.Split(out, "\n") {
			if strings.TrimSpace(l) != "" {
				outLines = append(outLines, l)
			}
		}

		Expect(outLines[0]).To(ContainSubstring("Login to your Epinio cluster"))
		Expect(outLines[0]).To(ContainSubstring(randomPort))

		Expect(outLines[1]).To(ContainSubstring("error while checking CA"))
		Expect(outLines[1]).To(ContainSubstring("%s: connect: connection refused", randomPort))
	})
})
