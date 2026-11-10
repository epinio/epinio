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
	"os"
	"path/filepath"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("validateLocalSourcePath", func() {
	var appDir string

	manifestFor := func(origin models.ApplicationOrigin, sourcePath string) models.ApplicationManifest {
		return models.ApplicationManifest{
			Origin:  origin,
			Staging: models.ApplicationStage{SourcePath: sourcePath},
		}
	}

	BeforeEach(func() {
		appDir = GinkgoT().TempDir()
		Expect(os.MkdirAll(filepath.Join(appDir, "apps", "web"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(appDir, "README.md"), []byte("x"), 0o600)).To(Succeed())
	})

	pathOrigin := func() models.ApplicationOrigin {
		return models.ApplicationOrigin{Kind: models.OriginPath, Path: appDir}
	}

	It("accepts an existing subdirectory of a local path origin", func() {
		Expect(validateLocalSourcePath(manifestFor(pathOrigin(), "apps/web"))).To(Succeed())
	})

	It("does nothing when no source path is set", func() {
		Expect(validateLocalSourcePath(manifestFor(pathOrigin(), ""))).To(Succeed())
	})

	It("treats '.' as the sources root", func() {
		Expect(validateLocalSourcePath(manifestFor(pathOrigin(), "."))).To(Succeed())
	})

	It("fails when the subdirectory does not exist", func() {
		err := validateLocalSourcePath(manifestFor(pathOrigin(), "apps/nope"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("source path not found"))
	})

	It("fails when the source path is a file", func() {
		err := validateLocalSourcePath(manifestFor(pathOrigin(), "README.md"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("is not a directory"))
	})

	It("rejects an invalid source path before touching the filesystem", func() {
		Expect(validateLocalSourcePath(manifestFor(pathOrigin(), "../escape"))).ToNot(Succeed())
		Expect(validateLocalSourcePath(manifestFor(pathOrigin(), "/etc"))).ToNot(Succeed())
	})

	It("skips the local check for non-path origins", func() {
		git := models.ApplicationOrigin{Kind: models.OriginGit}
		Expect(validateLocalSourcePath(manifestFor(git, "apps/nope"))).To(Succeed())
	})
})
