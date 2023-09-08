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

package cmd_test

import (
	"bytes"
	"io"
	"strings"

	"github.com/epinio/epinio/internal/cli/cmd"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/cli/usercmd"

	"github.com/spf13/cobra"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Command 'epinio settings'", func() {

	var (
		epinioClient      *usercmd.EpinioClient
		output, outputErr io.ReadWriter
		settingsCmd       *cobra.Command
	)

	BeforeEach(func() {
		var err error
		epinioClient, err = usercmd.New()
		Expect(err).To(BeNil())

		output = &bytes.Buffer{}
		outputErr = &bytes.Buffer{}
		epinioClient.UI().SetOutput(output)

		settingsCmd = cmd.NewSettingsCmd(epinioClient)
	})

	Describe("colors", func() {
		When("no argument is provided", func() {
			It("shows the usage and an error", func() {
				args := []string{"colors"}
				stdout, stderr, _ := executeCmd(settingsCmd, args, output, outputErr)
				Expect(stdout).To(HavePrefix("Usage:"))
				Expect(stderr).To(Equal("Error: accepts 1 arg(s), received 0\n"))
			})
		})

		When("an invalid value is provided", func() {
			It("shows the usage and an error", func() {
				args := []string{"colors", "foobar"}
				_, stderr, _ := executeCmd(settingsCmd, args, nil, outputErr)
				Expect(stderr).To(Equal("Error: requires a boolean argument (true/false)\n"))
			})
		})
	})

	Describe("show", func() {
		When("no flags are provided", func() {
			It("will hide sensible values", func() {
				epinioClient.Settings = &settings.Settings{
					Location:  "/my/local/settings",
					Namespace: "mynamespace",
					AppChart:  "default-app-chart",
					User:      "myuser",
					Password:  "mypassword",
					Token: settings.TokenSetting{
						AccessToken: "mytoken",
					},
					API:    "https://epinio.io",
					WSS:    "wss://epinio.io",
					Certs:  "-- CERT --",
					Colors: true,
				}

				args := []string{"show"}
				stdout, _, _ := executeCmd(settingsCmd, args, output, nil)

				lines := strings.Split(stdout, "\n")
				Expect(lines).To(HaveLen(15), stdout)

				Expect(lines[0]).To(Equal("🚢  Show Settings"))
				Expect(lines[1]).To(Equal("Settings: /my/local/settings"))
				Expect(lines[2]).To(Equal("✔️  Ok"))

				Expect(stdout).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("Colorized Output", "true"),
						WithRow("Current Namespace", "mynamespace"),
						WithRow("Default App Chart", "default-app-chart"),
						WithRow("API User Name", "myuser"),
						WithRow("API Password", "[*]+"),
						WithRow("API Token", "[*]+"),
						WithRow("API Url", "https://epinio.io"),
						WithRow("WSS Url", "wss://epinio.io"),
						WithRow("Certificates", "Present"),
					),
				)
			})
		})

		When("the password is set and the --show-password flag is set", func() {
			It("will show the value", func() {
				epinioClient.Settings = &settings.Settings{
					Password: "mypassword",
				}

				args := []string{"show", "--show-password"}
				stdout, _, _ := executeCmd(settingsCmd, args, output, nil)

				lines := strings.Split(stdout, "\n")
				Expect(lines).To(HaveLen(15), stdout)

				Expect(lines[0]).To(Equal("🚢  Show Settings"))
				Expect(lines[2]).To(Equal("✔️  Ok"))

				Expect(stdout).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("API Password", "mypassword"),
					),
				)
			})
		})

		When("the token is set and the --show-token flag is set", func() {
			It("will show the value", func() {
				epinioClient.Settings = &settings.Settings{
					Token: settings.TokenSetting{
						AccessToken: "mytoken",
					},
				}

				args := []string{"show", "--show-password"}
				stdout, _, _ := executeCmd(settingsCmd, args, output, nil)

				lines := strings.Split(stdout, "\n")
				Expect(lines).To(HaveLen(15), stdout)

				Expect(lines[0]).To(Equal("🚢  Show Settings"))
				Expect(lines[2]).To(Equal("✔️  Ok"))

				Expect(stdout).To(
					HaveATable(
						WithHeaders("KEY", "VALUE"),
						WithRow("API Token", "mytoken"),
					),
				)
			})
		})
	})
})
