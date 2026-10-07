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
	"encoding/base64"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/internal/cli/settings"
)

var _ = Describe("Settings password", func() {
	const secret = "s3cret/pass"

	var (
		encoded      = base64.StdEncoding.EncodeToString([]byte(secret))
		missingFile  string
		settingsFile string
	)

	BeforeEach(func() {
		dir := GinkgoT().TempDir()
		missingFile = filepath.Join(dir, "missing.yaml")
		settingsFile = filepath.Join(dir, "settings.yaml")

		// Start every case from a clean environment.
		for _, name := range []string{"EPINIO_PASSWORD", "EPINIO_PASSWORD_B64", "EPINIO_PASS"} {
			GinkgoT().Setenv(name, "")
		}
	})

	Describe("from the environment, without a settings file", func() {
		It("takes EPINIO_PASSWORD as plaintext", func() {
			GinkgoT().Setenv("EPINIO_PASSWORD", secret)

			cfg, err := settings.LoadFrom(missingFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Password).To(Equal(secret))
		})

		It("decodes EPINIO_PASSWORD_B64", func() {
			GinkgoT().Setenv("EPINIO_PASSWORD_B64", encoded)

			cfg, err := settings.LoadFrom(missingFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Password).To(Equal(secret))
		})

		It("still accepts EPINIO_PASS as an alias of EPINIO_PASSWORD_B64", func() {
			GinkgoT().Setenv("EPINIO_PASS", encoded)

			cfg, err := settings.LoadFrom(missingFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Password).To(Equal(secret))
		})

		It("fails when EPINIO_PASSWORD and EPINIO_PASSWORD_B64 are both set", func() {
			GinkgoT().Setenv("EPINIO_PASSWORD", secret)
			GinkgoT().Setenv("EPINIO_PASSWORD_B64", encoded)

			_, err := settings.LoadFrom(missingFile)
			Expect(err).To(MatchError(ContainSubstring("EPINIO_PASSWORD and EPINIO_PASSWORD_B64")))
		})

		It("fails when EPINIO_PASSWORD and the EPINIO_PASS alias are both set", func() {
			GinkgoT().Setenv("EPINIO_PASSWORD", secret)
			GinkgoT().Setenv("EPINIO_PASS", encoded)

			_, err := settings.LoadFrom(missingFile)
			Expect(err).To(HaveOccurred())
		})

		It("fails when EPINIO_PASSWORD_B64 is not valid base64", func() {
			GinkgoT().Setenv("EPINIO_PASSWORD_B64", "not base64!")

			_, err := settings.LoadFrom(missingFile)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("with a settings file", func() {
		BeforeEach(func() {
			Expect(os.WriteFile(settingsFile, []byte("pass: "+encoded+"\n"), 0600)).To(Succeed())
		})

		It("decodes the stored password", func() {
			cfg, err := settings.LoadFrom(settingsFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Password).To(Equal(secret))
		})

		It("lets EPINIO_PASSWORD override the stored password", func() {
			GinkgoT().Setenv("EPINIO_PASSWORD", "other")

			cfg, err := settings.LoadFrom(settingsFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Password).To(Equal("other"))
		})
	})
})

var _ = Describe("Settings API token", func() {
	var (
		missingFile  string
		settingsFile string
	)

	BeforeEach(func() {
		dir := GinkgoT().TempDir()
		missingFile = filepath.Join(dir, "missing.yaml")
		settingsFile = filepath.Join(dir, "settings.yaml")

		GinkgoT().Setenv("EPINIO_API_TOKEN", "")
	})

	It("is read from EPINIO_API_TOKEN, without a settings file", func() {
		GinkgoT().Setenv("EPINIO_API_TOKEN", "env-token")

		cfg, err := settings.LoadFrom(missingFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.Token.AccessToken).To(Equal("env-token"))
	})

	It("stays empty when EPINIO_API_TOKEN is not set", func() {
		cfg, err := settings.LoadFrom(missingFile)
		Expect(err).ToNot(HaveOccurred())
		Expect(cfg.Token.AccessToken).To(BeEmpty())
	})

	When("the settings file has a token", func() {
		BeforeEach(func() {
			content := "token:\n  accesstoken: file-token\n  refreshtoken: file-refresh\n"
			Expect(os.WriteFile(settingsFile, []byte(content), 0600)).To(Succeed())
		})

		It("keeps the stored token", func() {
			cfg, err := settings.LoadFrom(settingsFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Token.AccessToken).To(Equal("file-token"))
			Expect(cfg.Token.RefreshToken).To(Equal("file-refresh"))
		})

		It("lets EPINIO_API_TOKEN override the stored access token only", func() {
			GinkgoT().Setenv("EPINIO_API_TOKEN", "env-token")

			cfg, err := settings.LoadFrom(settingsFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Token.AccessToken).To(Equal("env-token"))
			Expect(cfg.Token.RefreshToken).To(Equal("file-refresh"))
		})
	})
})

var _ = Describe("Settings without a file", func() {
	var (
		missingFile  string
		settingsFile string
	)

	BeforeEach(func() {
		dir := GinkgoT().TempDir()
		missingFile = filepath.Join(dir, "missing.yaml")
		settingsFile = filepath.Join(dir, "settings.yaml")

		GinkgoT().Setenv("EPINIO_API", "")
	})

	Describe("EnvOnly", func() {
		It("is true without a file, when the API comes from the environment", func() {
			GinkgoT().Setenv("EPINIO_API", "https://epinio.example.com")

			cfg, err := settings.LoadFrom(missingFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.EnvOnly()).To(BeTrue())
		})

		It("is false without a file and without an API", func() {
			cfg, err := settings.LoadFrom(missingFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.EnvOnly()).To(BeFalse())
		})

		It("is false with a file, even when the API is overridden by the environment", func() {
			Expect(os.WriteFile(settingsFile, []byte("user: admin\n"), 0600)).To(Succeed())
			GinkgoT().Setenv("EPINIO_API", "https://epinio.example.com")

			cfg, err := settings.LoadFrom(settingsFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.EnvOnly()).To(BeFalse())
		})
	})

	Describe("Origin", func() {
		It("names the environment", func() {
			GinkgoT().Setenv("EPINIO_API", "https://epinio.example.com")

			cfg, err := settings.LoadFrom(missingFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Origin()).To(Equal("<environment>"))
		})

		It("is the absolute path of the settings file", func() {
			Expect(os.WriteFile(settingsFile, []byte("user: admin\n"), 0600)).To(Succeed())

			cfg, err := settings.LoadFrom(settingsFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Origin()).To(Equal(settingsFile))
		})
	})

	Describe("SaveUnlessEnvOnly", func() {
		It("refuses to write a file for settings from the environment", func() {
			GinkgoT().Setenv("EPINIO_API", "https://epinio.example.com")
			GinkgoT().Setenv("EPINIO_PASSWORD", "secret")

			cfg, err := settings.LoadFrom(missingFile)
			Expect(err).ToNot(HaveOccurred())

			Expect(cfg.SaveUnlessEnvOnly()).To(MatchError(settings.ErrEnvOnly))
			Expect(missingFile).ToNot(BeAnExistingFile())
		})

		It("saves settings which came from a file", func() {
			Expect(os.WriteFile(settingsFile, []byte("user: admin\n"), 0600)).To(Succeed())

			cfg, err := settings.LoadFrom(settingsFile)
			Expect(err).ToNot(HaveOccurred())
			cfg.Namespace = "targeted"

			Expect(cfg.SaveUnlessEnvOnly()).To(Succeed())

			content, err := os.ReadFile(settingsFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("namespace: targeted"))
		})

		It("still creates the file when nothing comes from the environment", func() {
			cfg, err := settings.LoadFrom(missingFile)
			Expect(err).ToNot(HaveOccurred())
			cfg.Colors = false

			Expect(cfg.SaveUnlessEnvOnly()).To(Succeed())
			Expect(missingFile).To(BeAnExistingFile())
		})
	})
})
