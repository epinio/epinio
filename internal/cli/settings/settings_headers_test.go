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

package settings_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/epinio/epinio/internal/cli/settings"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSettings(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Settings Suite")
}

var _ = Describe("Settings Headers", func() {
	var tmpFile string

	BeforeEach(func() {
		dir, err := os.MkdirTemp("", "epinio-settings-test-*")
		Expect(err).ToNot(HaveOccurred())
		tmpFile = filepath.Join(dir, "settings.yaml")
	})

	AfterEach(func() {
		Expect(os.RemoveAll(filepath.Dir(tmpFile))).To(Succeed())
	})

	It("preserves headers across Save/Load and writes the file as 0600", func() {
		cfg, err := settings.LoadFrom(tmpFile)
		Expect(err).ToNot(HaveOccurred())

		cfg.API = "https://epinio.example.com"
		cfg.User = "admin"
		cfg.Password = "password"
		cfg.Headers = http.Header{
			"Proxy-Authorization": []string{"Bearer test-token-123"},
		}

		Expect(cfg.Save()).To(Succeed())

		info, err := os.Stat(tmpFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))

		loaded, err := settings.LoadFrom(tmpFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(loaded.Headers.Get("Proxy-Authorization")).To(Equal("Bearer test-token-123"))
	})

	It("clears headers on Save when Headers is nil", func() {
		cfg, err := settings.LoadFrom(tmpFile)
		Expect(err).ToNot(HaveOccurred())
		cfg.API = "https://epinio.example.com"
		cfg.Headers = http.Header{
			"Proxy-Authorization": []string{"Bearer keep-me"},
		}
		Expect(cfg.Save()).To(Succeed())

		cfg.Headers = nil
		Expect(cfg.Save()).To(Succeed())

		loaded, err := settings.LoadFrom(tmpFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(loaded.Headers).To(Or(BeNil(), HaveLen(0)))
	})
})
