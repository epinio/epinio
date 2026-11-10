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
	"path/filepath"

	"github.com/epinio/epinio/internal/manifest"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("Manifest source path", func() {
	Describe("Get", func() {
		write := func(content string) string {
			file := filepath.Join(GinkgoT().TempDir(), "manifest.yml")
			Expect(os.WriteFile(file, []byte(content), 0o600)).To(Succeed())
			return file
		}

		It("reads and normalizes staging.sourcePath", func() {
			m, err := manifest.Get(write("name: foo\nstaging:\n  sourcePath: ./apps/web/\n"))
			Expect(err).ToNot(HaveOccurred())
			Expect(m.Staging.SourcePath).To(Equal("apps/web"))
		})

		It("leaves the source path empty when not set", func() {
			m, err := manifest.Get(write("name: foo\n"))
			Expect(err).ToNot(HaveOccurred())
			Expect(m.Staging.SourcePath).To(BeEmpty())
		})

		It("rejects an invalid staging.sourcePath", func() {
			_, err := manifest.Get(write("name: foo\nstaging:\n  sourcePath: ../escape\n"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid staging.sourcePath"))
		})
	})

	Describe("UpdateBuilder --source-path", func() {
		newCmd := func() *cobra.Command {
			c := &cobra.Command{}
			c.Flags().String("builder-image", "", "")
			c.Flags().String("build-mode", "", "")
			c.Flags().String("dockerfile-path", "", "")
			c.Flags().String("source-path", "", "")
			return c
		}

		withStored := func() models.ApplicationManifest {
			return models.ApplicationManifest{
				Staging: models.ApplicationStage{SourcePath: "apps/old"},
			}
		}

		It("sets the validated source path from the flag", func() {
			c := newCmd()
			Expect(c.Flags().Set("source-path", "./apps/web/")).To(Succeed())

			m, err := manifest.UpdateBuilder(withStored(), c)
			Expect(err).ToNot(HaveOccurred())
			Expect(m.Staging.SourcePath).To(Equal("apps/web"))
		})

		It("keeps the manifest value when the flag is not given", func() {
			m, err := manifest.UpdateBuilder(withStored(), newCmd())
			Expect(err).ToNot(HaveOccurred())
			Expect(m.Staging.SourcePath).To(Equal("apps/old"))
		})

		It("keeps '.' so an explicit reset to the sources root reaches the server", func() {
			c := newCmd()
			Expect(c.Flags().Set("source-path", ".")).To(Succeed())

			m, err := manifest.UpdateBuilder(withStored(), c)
			Expect(err).ToNot(HaveOccurred())
			Expect(m.Staging.SourcePath).To(Equal(models.SourcePathRoot))
		})

		It("rejects an invalid flag value", func() {
			c := newCmd()
			Expect(c.Flags().Set("source-path", "../escape")).To(Succeed())

			_, err := manifest.UpdateBuilder(withStored(), c)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid --source-path"))
		})
	})
})
