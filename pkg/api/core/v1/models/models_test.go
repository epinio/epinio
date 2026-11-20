// Copyright © 2021 - 2026 KRUMWARE LLC
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
	"encoding/json"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
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

var _ = Describe("ApplicationManifest YAML", func() {
	It("exports bound configurations with their types", func() {
		m := models.ApplicationManifest{
			Name: "myapp",
			Configuration: models.ApplicationConfiguration{
				Configurations: []string{"mycfg", "mysvc-cfg"},
				BoundConfigurations: []models.BoundConfiguration{
					{Name: "mycfg", Type: "custom"},
					{Name: "mysvc-cfg", Type: "service"},
				},
			},
		}

		out, err := yaml.Marshal(m)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(out)).To(Equal(`name: myapp
configuration:
  configurations:
  - mycfg
  - mysvc-cfg
  bound_configurations:
  - name: mycfg
    type: custom
  - name: mysvc-cfg
    type: service
`))
	})

	It("omits bound configurations when there are none", func() {
		m := models.ApplicationManifest{
			Name: "myapp",
			Configuration: models.ApplicationConfiguration{
				BoundConfigurations: []models.BoundConfiguration{},
			},
		}

		out, err := yaml.Marshal(m)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(out)).ToNot(ContainSubstring("bound_configurations"))
	})

	It("reads bound configurations back without feeding them to push", func() {
		// The field round-trips so a re-read manifest is not lossy, but push binds
		// from `configurations` only -- see NewApplicationUpdateRequest.
		var m models.ApplicationManifest
		err := yaml.Unmarshal([]byte(`name: myapp
configuration:
  configurations:
  - mycfg
  bound_configurations:
  - name: mycfg
    type: custom
`), &m)
		Expect(err).ToNot(HaveOccurred())
		Expect(m.Configuration.BoundConfigurations).To(HaveLen(1))
		Expect(m.Configuration.BoundConfigurations[0].Name).To(Equal("mycfg"))
		Expect(m.Configuration.BoundConfigurations[0].Type).To(Equal("custom"))

		update := models.NewApplicationUpdateRequest(m)
		Expect(update.Configurations).To(Equal([]string{"mycfg"}))
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

var _ = Describe("ValidateSourcePath", func() {
	It("keeps empty distinct from an explicit sources-root reset", func() {
		path, err := models.ValidateSourcePath("")
		Expect(err).ToNot(HaveOccurred())
		Expect(path).To(Equal(""))

		path, err = models.ValidateSourcePath(".")
		Expect(err).ToNot(HaveOccurred())
		Expect(path).To(Equal(models.SourcePathRoot))

		path, err = models.ValidateSourcePath("./")
		Expect(err).ToNot(HaveOccurred())
		Expect(path).To(Equal(models.SourcePathRoot))
	})

	It("accepts relative subdirectory paths", func() {
		path, err := models.ValidateSourcePath("apps/web-app")
		Expect(err).ToNot(HaveOccurred())
		Expect(path).To(Equal("apps/web-app"))

		path, err = models.ValidateSourcePath("./apps/web-app/")
		Expect(err).ToNot(HaveOccurred())
		Expect(path).To(Equal("apps/web-app"))
	})

	It("rejects absolute paths and parent directory traversal", func() {
		_, err := models.ValidateSourcePath("/etc/passwd")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("absolute"))

		_, err = models.ValidateSourcePath("../apps")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(".."))

		_, err = models.ValidateSourcePath("foo/../../bar")
		Expect(err).To(HaveOccurred())
	})

	It("rejects unexpected characters", func() {
		_, err := models.ValidateSourcePath("my app")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("only letters"))

		_, err = models.ValidateSourcePath("apps/*")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("NormalizeSourcePathForStaging", func() {
	It("maps root sentinel and empty to the staging empty value", func() {
		Expect(models.NormalizeSourcePathForStaging("")).To(Equal(""))
		Expect(models.NormalizeSourcePathForStaging(models.SourcePathRoot)).To(Equal(""))
		Expect(models.NormalizeSourcePathForStaging("apps/web")).To(Equal("apps/web"))
	})
})

var _ = Describe("ValidateSourcePathForBuildMode", func() {
	It("allows source path with buildpack or empty build mode", func() {
		Expect(models.ValidateSourcePathForBuildMode("", "apps/web")).To(Succeed())
		Expect(models.ValidateSourcePathForBuildMode(models.BuildModeBuildpack, "apps/web")).To(Succeed())
		Expect(models.ValidateSourcePathForBuildMode(models.BuildModeDockerfile, "")).To(Succeed())
	})

	It("rejects source path with dockerfile build mode", func() {
		err := models.ValidateSourcePathForBuildMode(models.BuildModeDockerfile, "apps/web")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("buildpack staging"))
		Expect(err.Error()).To(ContainSubstring(models.BuildModeDockerfile))
	})
})

var _ = Describe("AppBuildStatus", func() {
	It("defines distinct create, build, and deployed values", func() {
		Expect(models.AppBuildStatusCreate).To(Equal("create"))
		Expect(models.AppBuildStatusBuild).To(Equal("build"))
		Expect(models.AppBuildStatusDeployed).To(Equal("deployed"))
		Expect(models.AppBuildStatusCreate).NotTo(Equal(models.AppBuildStatusBuild))
		Expect(models.AppBuildStatusBuild).NotTo(Equal(models.AppBuildStatusDeployed))
	})
})

var _ = Describe("AppLifecycleResult", func() {
	It("defines distinct succeeded and failed values for CR status stamps", func() {
		Expect(models.AppLifecycleSucceeded).To(Equal("succeeded"))
		Expect(models.AppLifecycleFailed).To(Equal("failed"))
		Expect(models.AppLifecycleSucceeded).NotTo(Equal(models.AppLifecycleFailed))
	})
})

var _ = Describe("LifecycleTiming JSON", func() {
	It("marshals with CRD field names", func() {
		timing := models.LifecycleTiming{
			DurationMs: 12345,
			FinishedAt: "2026-11-09T10:15:30Z",
			Result:     "succeeded",
			StageID:    "stage-build-001",
		}

		raw, err := json.Marshal(timing)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())
		Expect(got).To(HaveKey("durationms"))
		Expect(got).To(HaveKey("finishedat"))
		Expect(got).To(HaveKey("result"))
		Expect(got).To(HaveKey("stageid"))
		Expect(got).NotTo(HaveKey("durationMs"))
		Expect(got).NotTo(HaveKey("finishedAt"))
		Expect(got).NotTo(HaveKey("stage_id"))
		Expect(got).NotTo(HaveKey("stageID"))
		Expect(got["durationms"]).To(BeEquivalentTo(12345))
		Expect(got["finishedat"]).To(Equal("2026-11-09T10:15:30Z"))
		Expect(got["result"]).To(Equal("succeeded"))
		Expect(got["stageid"]).To(Equal("stage-build-001"))
	})

	It("omits an empty stageid", func() {
		timing := models.LifecycleTiming{
			DurationMs: 10,
			FinishedAt: "2026-11-09T10:15:30Z",
			Result:     "failed",
		}

		raw, err := json.Marshal(timing)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())
		Expect(got).NotTo(HaveKey("stageid"))
		Expect(got["result"]).To(Equal("failed"))
	})

	It("round-trips through JSON without losing fields", func() {
		in := models.LifecycleTiming{
			DurationMs: 678,
			FinishedAt: "2026-11-09T10:20:00Z",
			Result:     "failed",
			StageID:    "stage-deploy-002",
		}

		raw, err := json.Marshal(in)
		Expect(err).ToNot(HaveOccurred())

		var out models.LifecycleTiming
		Expect(json.Unmarshal(raw, &out)).To(Succeed())
		Expect(out).To(Equal(in))
	})
})

var _ = Describe("App lifecycle timing fields", func() {
	It("omits lastBuild and lastDeploy from the App JSON when unset", func() {
		app := models.App{
			BuildStatus: models.AppBuildStatusCreate,
		}

		raw, err := json.Marshal(app)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())
		Expect(got).NotTo(HaveKey("lastBuild"))
		Expect(got).NotTo(HaveKey("lastDeploy"))
	})

	It("includes lastBuild and lastDeploy in the App JSON when set", func() {
		app := models.App{
			LastBuild: &models.LifecycleTiming{
				DurationMs: 100,
				FinishedAt: "2026-11-09T10:15:30Z",
				Result:     "succeeded",
				StageID:    "stage-1",
			},
			LastDeploy: &models.LifecycleTiming{
				DurationMs: 50,
				FinishedAt: "2026-11-09T10:16:00Z",
				Result:     "succeeded",
				StageID:    "stage-1",
			},
		}

		raw, err := json.Marshal(app)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())
		Expect(got).To(HaveKey("lastBuild"))
		Expect(got).To(HaveKey("lastDeploy"))

		lastBuild := got["lastBuild"].(map[string]any)
		Expect(lastBuild["durationms"]).To(BeEquivalentTo(100))
		Expect(lastBuild).NotTo(HaveKey("durationMs"))

		lastDeploy := got["lastDeploy"].(map[string]any)
		Expect(lastDeploy["durationms"]).To(BeEquivalentTo(50))
		Expect(lastDeploy["stageid"]).To(Equal("stage-1"))
	})
})

var _ = Describe("PodInfo StartupMs JSON", func() {
	It("omits startupMs when unset", func() {
		pod := models.PodInfo{
			Name:     "app-0",
			Ready:    true,
			Restarts: 0,
		}

		raw, err := json.Marshal(pod)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())
		Expect(got).NotTo(HaveKey("startupMs"))
	})

	It("includes startupMs when set", func() {
		startupMs := int64(1500)
		pod := models.PodInfo{
			Name:      "app-0",
			Ready:     true,
			Restarts:  0,
			StartupMs: &startupMs,
		}

		raw, err := json.Marshal(pod)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())
		Expect(got).To(HaveKey("startupMs"))
		Expect(got["startupMs"]).To(BeEquivalentTo(1500))
	})

	It("round-trips a zero startupMs when explicitly set", func() {
		startupMs := int64(0)
		pod := models.PodInfo{
			Name:      "app-0",
			StartupMs: &startupMs,
		}

		raw, err := json.Marshal(pod)
		Expect(err).ToNot(HaveOccurred())

		var out models.PodInfo
		Expect(json.Unmarshal(raw, &out)).To(Succeed())
		Expect(out.StartupMs).ToNot(BeNil())
		Expect(*out.StartupMs).To(Equal(int64(0)))
	})
})
