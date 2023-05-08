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

package machine

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/auth"
	. "github.com/epinio/epinio/acceptance/helpers/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (m *Machine) ExpectGoodUserLogin(tmpSettingsPath, password, serverURL string) {
	out, err := m.Epinio("", "login", "-u", "epinio", "-p", password,
		"--trust-ca", "--settings-file", tmpSettingsPath, serverURL)

	Expect(err).ToNot(HaveOccurred())
	Expect(out).To(ContainSubstring(`Login to your Epinio cluster`))
	Expect(out).To(ContainSubstring(`Trusting certificate`))
	Expect(out).To(ContainSubstring(`Login successful`))
}

func (m *Machine) ExpectGoodTokenLogin(tmpSettingsPath, serverURL string) {

	out := &bytes.Buffer{}
	cmd := m.EpinioCmd("login", "--prompt", "--oidc",
		"--trust-ca", "--settings-file", tmpSettingsPath, serverURL)
	cmd.Stdout = out
	cmd.Stderr = out

	stdinPipe, err := cmd.StdinPipe()
	Expect(err).ToNot(HaveOccurred())

	// run the epinio login and wait for the input of the authCode
	go func() {
		defer GinkgoRecover()

		err = cmd.Run()
		Expect(err).ToNot(HaveOccurred(), out.String())

		// when the command terminates check that the login was successful
		Expect(out.String()).To(ContainSubstring(`Login successful`))

		// check that the settings are now updated
		m.ExpectTokenSettings(tmpSettingsPath)
	}()

	// read the full output, until the command asks you to paste the auth code
	for {
		if strings.Contains(out.String(), "paste the authorization code") {
			break
		}
	}

	fullOutput := out.String()

	Expect(fullOutput).To(ContainSubstring(`Login to your Epinio cluster`))
	Expect(fullOutput).To(ContainSubstring(`Trusting certificate`))

	lines := strings.Split(fullOutput, "\n")

	var authURL string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://auth") {
			authURL = line
			break
		}
	}
	Expect(authURL).ToNot(BeEmpty())

	// authenticate with Dex, get the authCode and submit the input to the waiting command
	u, err := url.Parse(authURL)
	Expect(err).ToNot(HaveOccurred())
	loginClient, err := auth.NewDexClient(fmt.Sprintf("%s://%s", u.Scheme, u.Host))
	Expect(err).ToNot(HaveOccurred())

	authCode, err := loginClient.Login(authURL, "admin@epinio.io", "password")
	Expect(err).ToNot(HaveOccurred())
	_, err = fmt.Fprintln(stdinPipe, authCode)
	Expect(err).ToNot(HaveOccurred())

}

func (m *Machine) ExpectEmptySettings(tmpSettingsPath string) {
	// check that the settings are empty
	settings, err := m.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
	Expect(err).ToNot(HaveOccurred(), settings)
	Expect(settings).To(
		HaveATable(
			WithHeaders("KEY", "VALUE"),
			WithRow("API User Name", ""),
			WithRow("API Password", ""),
			WithRow("API Token", ""),
			WithRow("Certificates", "None defined"),
		),
	)
}

func (m *Machine) ExpectUserPasswordSettings(tmpSettingsPath string) {
	// check that the settings contain user/assword authentication
	settings, err := m.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
	Expect(err).ToNot(HaveOccurred(), settings)
	Expect(settings).To(
		HaveATable(
			WithHeaders("KEY", "VALUE"),
			WithRow("API User Name", "epinio"),
			WithRow("API Password", "[*]+"),
			WithRow("API Token", ""),
			WithRow("Certificates", "Present"),
		),
	)
}

func (m *Machine) ExpectTokenSettings(tmpSettingsPath string) {
	// check that the settings contain user/assword authentication
	settings, err := m.Epinio("", "settings", "show", "--settings-file", tmpSettingsPath)
	Expect(err).ToNot(HaveOccurred(), settings)
	Expect(settings).To(
		HaveATable(
			WithHeaders("KEY", "VALUE"),
			WithRow("API User Name", "epinio"),
			WithRow("API Password", ""),
			WithRow("API Token", "[*]+"),
			WithRow("Certificates", "Present"),
		),
	)
}
