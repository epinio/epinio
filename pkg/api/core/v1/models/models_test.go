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

package models_test

import (
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ApplicationOrigin String()", func() {
	It("correctly reports a bogus origin kind", func() {
		o := &models.ApplicationOrigin{
			Kind: 22,
		}
		s := o.String()
		Expect(s).To(Equal("<<undefined>>"))
	})

	It("correctly stringifies path origin", func() {
		o := &models.ApplicationOrigin{
			Kind: models.OriginPath,
			Path: "/foo",
		}
		s := o.String()
		Expect(s).To(Equal(helpers.AbsPath("/foo")))
	})

	It("correctly stringifies archive path origin", func() {
		o := &models.ApplicationOrigin{
			Kind:    models.OriginPath,
			Archive: true,
			Path:    "/foo",
		}
		s := o.String()
		Expect(s).To(Equal(helpers.AbsPath("/foo") + " (archive)"))
	})

	It("correctly stringifies a container origin", func() {
		o := &models.ApplicationOrigin{
			Kind:      models.OriginContainer,
			Container: "snafu",
		}
		s := o.String()
		Expect(s).To(Equal("snafu"))
	})

	// git: 4 cases :: +/- revision, +/- branch, url is mandatory
	It("correctly stringifies a git origin, url only", func() {
		o := &models.ApplicationOrigin{
			Kind: models.OriginGit,
			Git: &models.GitRef{
				URL: "somewhere",
			},
		}
		s := o.String()
		Expect(s).To(Equal("somewhere"))
	})

	It("correctly stringifies a git origin, url + revision", func() {
		o := &models.ApplicationOrigin{
			Kind: models.OriginGit,
			Git: &models.GitRef{
				URL:      "somewhere",
				Revision: "efi258945kda",
			},
		}
		s := o.String()
		Expect(s).To(Equal("somewhere @ efi258945kda"))
	})

	It("correctly stringifies a git origin, url + branch", func() {
		o := &models.ApplicationOrigin{
			Kind: models.OriginGit,
			Git: &models.GitRef{
				URL:    "somewhere",
				Branch: "subtotal",
			},
		}
		s := o.String()
		Expect(s).To(Equal("somewhere on subtotal"))
	})

	It("correctly stringifies a git origin, url + revision and branch", func() {
		o := &models.ApplicationOrigin{
			Kind: models.OriginGit,
			Git: &models.GitRef{
				URL:      "somewhere",
				Revision: "efi258945kda",
				Branch:   "subtotal",
			},
		}
		s := o.String()
		Expect(s).To(Equal("somewhere @ efi258945kda (on subtotal)"))
	})

	Describe("NewApplicationUpdateRequest", func() {
		It("copies ReplaceEnv when set", func() {
			replace := true
			m := models.ApplicationManifest{
				Configuration: models.ApplicationConfiguration{
					ReplaceEnv: &replace,
				},
			}

			update := models.NewApplicationUpdateRequest(m)
			Expect(update.ReplaceEnv).ToNot(BeNil())
			Expect(*update.ReplaceEnv).To(BeTrue())
		})

		It("keeps ReplaceEnv nil when unset", func() {
			m := models.ApplicationManifest{}

			update := models.NewApplicationUpdateRequest(m)
			Expect(update.ReplaceEnv).To(BeNil())
		})
	})
})

var _ = Describe("ValidateBuildMode", func() {
	It("defaults empty to buildpack", func() {
		mode, err := models.ValidateBuildMode("")
		Expect(err).ToNot(HaveOccurred())
		Expect(mode).To(Equal(models.BuildModeBuildpack))
	})

	It("accepts buildpack and dockerfile case-insensitively", func() {
		mode, err := models.ValidateBuildMode("BuildPack")
		Expect(err).ToNot(HaveOccurred())
		Expect(mode).To(Equal(models.BuildModeBuildpack))

		mode, err = models.ValidateBuildMode("DockerFile")
		Expect(err).ToNot(HaveOccurred())
		Expect(mode).To(Equal(models.BuildModeDockerfile))
	})

	It("rejects typos instead of mapping them to buildpack", func() {
		mode, err := models.ValidateBuildMode("dockerfle")
		Expect(err).To(HaveOccurred())
		Expect(mode).To(Equal(""))
		Expect(err.Error()).To(ContainSubstring("invalid build mode"))
		Expect(models.NormalizeBuildMode("dockerfle")).ToNot(Equal(models.BuildModeBuildpack))
	})
})

var _ = Describe("ValidateDockerfilePath", func() {
	It("defaults empty to Dockerfile", func() {
		path, err := models.ValidateDockerfilePath("")
		Expect(err).ToNot(HaveOccurred())
		Expect(path).To(Equal("Dockerfile"))
	})

	It("accepts relative paths within app sources", func() {
		path, err := models.ValidateDockerfilePath("Dockerfile")
		Expect(err).ToNot(HaveOccurred())
		Expect(path).To(Equal("Dockerfile"))

		path, err = models.ValidateDockerfilePath("docker/Dockerfile")
		Expect(err).ToNot(HaveOccurred())
		Expect(path).To(Equal("docker/Dockerfile"))
	})

	It("rejects absolute paths and parent directory traversal", func() {
		_, err := models.ValidateDockerfilePath("/etc/passwd")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("absolute"))

		_, err = models.ValidateDockerfilePath("../Dockerfile")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(".."))

		_, err = models.ValidateDockerfilePath("foo/../../bar")
		Expect(err).To(HaveOccurred())
	})

	It("rejects unexpected characters", func() {
		_, err := models.ValidateDockerfilePath("my Dockerfile")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("only letters"))

		_, err = models.ValidateDockerfilePath("foo*.Dockerfile")
		Expect(err).To(HaveOccurred())
	})
})
