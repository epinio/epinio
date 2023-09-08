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

package usercmd_test

import (
	"bytes"
	"io"
	"strings"

	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/internal/cli/usercmd"
	"github.com/epinio/epinio/internal/cli/usercmd/usercmdfakes"

	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client Settings unit tests", func() {

	var (
		fake   *usercmdfakes.FakeAPIClient
		output io.ReadWriter
	)

	Describe("SettingsShow", func() {

		BeforeEach(func() {
			fake = &usercmdfakes.FakeAPIClient{}
			output = &bytes.Buffer{}
		})

		When("restaging an existing app", func() {

			It("returns no error", func() {
				epinioClient, err := usercmd.New()
				Expect(err).ToNot(HaveOccurred())

				epinioClient.UI().SetOutput(output)
				epinioClient.API = fake

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

				var showPassword, showToken bool
				epinioClient.SettingsShow(showPassword, showToken)

				stdout, err := io.ReadAll(output)
				Expect(err).ToNot(HaveOccurred())

				lines := strings.Split(string(stdout), "\n")
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
	})
})
