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
	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logout", LMisc, func() {
	var tmpSettingsPath string

	BeforeEach(func() {
		tmpSettingsPath = catalog.NewTmpName("tmpEpinio") + `.yaml`
	})

	AfterEach(func() {
		// Remove transient settings
		out, err := proc.Run("", false, "rm", "-f", tmpSettingsPath)
		Expect(err).ToNot(HaveOccurred(), out)
	})

	It("succeeds after regular login", func() {
		// check that the initial settings are empty
		ExpectEmptySettings(tmpSettingsPath)

		// login with a valid user
		ExpectGoodUserLogin(tmpSettingsPath, env.EpinioPassword, serverURL)

		// check that the settings are now updated
		ExpectUserPasswordSettings(tmpSettingsPath)

		// logout
		_, err := env.Epinio("", "logout", "--settings-file", tmpSettingsPath)
		Expect(err).ToNot(HaveOccurred())

		// check that the settings are empty again
		ExpectEmptySettings(tmpSettingsPath)
	})

	It("succeeds after a login with OIDC", func() {
		// check that the initial settings are empty
		ExpectEmptySettings(tmpSettingsPath)

		// login via oidc
		ExpectGoodTokenLogin(tmpSettingsPath, serverURL)

		// logout again
		_, err := env.Epinio("", "logout", "--settings-file", tmpSettingsPath)
		Expect(err).ToNot(HaveOccurred())

		// check that the settings are empty again
		ExpectEmptySettings(tmpSettingsPath)
	})
})
