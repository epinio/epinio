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

package usercmd

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/epinio/epinio/internal/cli/settings"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("persistLoginHeaders", func() {
	var (
		epinioSettings  *settings.Settings
		incomingHeaders http.Header
	)

	BeforeEach(func() {
		epinioSettings = &settings.Settings{}
		incomingHeaders = http.Header{
			"Proxy-Authorization": []string{"Bearer test-token-123"},
		}
	})

	It("stores headers when remember is true and headers are present", func() {
		result := persistLoginHeaders(epinioSettings, incomingHeaders, true)
		Expect(result).To(Equal(headerPersistSaved))
		Expect(epinioSettings.Headers.Get("Proxy-Authorization")).To(Equal("Bearer test-token-123"))
	})

	It("clears headers when remember is false", func() {
		epinioSettings.Headers = http.Header{
			"Proxy-Authorization": []string{"Bearer old-token"},
		}
		result := persistLoginHeaders(epinioSettings, incomingHeaders, false)
		Expect(result).To(Equal(headerPersistCleared))
		Expect(epinioSettings.Headers).To(BeNil())
	})

	It("clears headers when remember is true but incoming headers are empty", func() {
		epinioSettings.Headers = http.Header{
			"Proxy-Authorization": []string{"Bearer old-token"},
		}
		result := persistLoginHeaders(epinioSettings, http.Header{}, true)
		Expect(result).To(Equal(headerPersistCleared))
		Expect(epinioSettings.Headers).To(BeNil())
	})

	It("reports remember-without-headers when nothing was stored", func() {
		result := persistLoginHeaders(epinioSettings, http.Header{}, true)
		Expect(result).To(Equal(headerPersistRememberNoHeaders))
		Expect(epinioSettings.Headers).To(BeNil())
	})

	It("reports none when remember is false and nothing was stored", func() {
		result := persistLoginHeaders(epinioSettings, http.Header{}, false)
		Expect(result).To(Equal(headerPersistNone))
		Expect(epinioSettings.Headers).To(BeNil())
	})
})

var _ = Describe("clearAuthSettings", func() {
	var tmpFile string

	BeforeEach(func() {
		dir, err := os.MkdirTemp("", "epinio-logout-headers-*")
		Expect(err).ToNot(HaveOccurred())
		tmpFile = filepath.Join(dir, "settings.yaml")

		cfg, err := settings.LoadFrom(tmpFile)
		Expect(err).ToNot(HaveOccurred())
		cfg.API = "https://epinio.example.com"
		cfg.User = "admin"
		cfg.Password = "password"
		cfg.Headers = http.Header{
			"Proxy-Authorization": []string{"Bearer test-token-123"},
		}
		Expect(cfg.Save()).To(Succeed())

		viper.Set("settings-file", tmpFile)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(filepath.Dir(tmpFile))).To(Succeed())
	})

	It("clears stored headers", func() {
		cleared, err := clearAuthSettings()
		Expect(err).ToNot(HaveOccurred())
		Expect(cleared.Headers).To(BeNil())
		Expect(cleared.User).To(BeEmpty())
		Expect(cleared.Password).To(BeEmpty())
		Expect(cleared.API).To(BeEmpty())
	})
})
