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

package manifest_test

import (
	"os"
	"path"

	"github.com/epinio/epinio/internal/manifest"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manifest", func() {
	var workdir string

	BeforeEach(func() {
		var err error
		workdir, err = os.Getwd()
		Expect(err).ToNot(HaveOccurred(), workdir)
	})

	Describe("Get", func() {
		When("the desired manifest file is missing", func() {
			It("returns defaults", func() {
				m, err := manifest.Get("missing.yml")
				Expect(err).ToNot(HaveOccurred())
				Expect(m).To(Equal(models.ApplicationManifest{
					Self: "<<Defaults>>",
					Origin: models.ApplicationOrigin{
						Kind:      models.OriginPath,
						Path:      workdir,
						Container: "",
					},
					Staging: models.ApplicationStage{},
				}))
			})
		})

		When("the desired manifest file is not accessible", func() {
			BeforeEach(func() {
				f, err := os.Create("unreadable.yml")
				Expect(err).ToNot(HaveOccurred())
				err = f.Chmod(0)
				Expect(err).ToNot(HaveOccurred())
				f.Close()
			})

			AfterEach(func() {
				err := os.Remove("unreadable.yml")
				Expect(err).ToNot(HaveOccurred())
			})

			It("fails with an error", func() {
				_, err := manifest.Get("unreadable.yml")
				Expect(err.Error()).
					To(MatchRegexp(`open .*/unreadable.yml: permission denied`))
			})
		})

		When("the desired manifest file does not contain proper YAML", func() {
			BeforeEach(func() {
				err := os.WriteFile("badyaml.yml", []byte(`a: b: c
d: e`), 0600)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				err := os.Remove("badyaml.yml")
				Expect(err).ToNot(HaveOccurred())
			})

			It("fails with an error", func() {
				_, err := manifest.Get("badyaml.yml")
				//Expect(err).ToNot(HaveOccurred())
				Expect(err.Error()).To(Equal(`bad yaml: yaml: mapping values are not allowed in this context`))
			})
		})

		When("the desired manifest file does contain proper YAML", func() {
			BeforeEach(func() {
				err := os.WriteFile("goodyaml.yml", []byte(`name: foo
staging:
  builder: snafu
origin:
  git:
    revision: off
    url: kilter
configuration:
  instances: 2
  configurations:
  - bar
  environment:
    CREDO: up
    DOGMA: "no"
`), 0600)
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				err := os.Remove("goodyaml.yml")
				Expect(err).ToNot(HaveOccurred())
			})

			It("works", func() {
				m, err := manifest.Get("goodyaml.yml")
				Expect(err).ToNot(HaveOccurred())
				var instances int32 = 2
				Expect(m).To(Equal(models.ApplicationManifest{
					Name: "foo",
					Configuration: models.ApplicationConfiguration{
						Instances: &instances,
						Configurations: []string{
							"bar",
						},
						Environment: models.EnvVariableMap{
							"DOGMA": "no",
							"CREDO": "up",
						},
					},
					Self: path.Join(workdir, "goodyaml.yml"),
					Origin: models.ApplicationOrigin{
						Kind:      models.OriginGit,
						Path:      "",
						Container: "",
						Git: &models.GitRef{
							Revision: "off",
							URL:      "kilter",
						},
					},
					Staging: models.ApplicationStage{
						Builder: "snafu",
					},
				}))

			})
		})
	})
})
