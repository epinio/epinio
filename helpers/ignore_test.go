// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package helpers_test

import (
	"os"
	"path/filepath"

	"github.com/epinio/epinio/helpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IgnoreMatcher", func() {
	var tempDir string
	var matcher *helpers.IgnoreMatcher

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "epinio-ignore-test-*")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Context("when .epinioignore file exists", func() {
		BeforeEach(func() {
			ignoreContent := `node_modules/
				*.log
				dist/
				.env
				# This is a comment
				!important.log
			`
			ignoreFile := filepath.Join(tempDir, ".epinioignore")
			err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			var err2 error
			matcher, err2 = helpers.LoadIgnoreMatcher(tempDir, nil)
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should ignore node_modules directory", func() {
			nodeModulesPath := filepath.Join(tempDir, "node_modules")
			Expect(matcher.ShouldIgnore(tempDir, nodeModulesPath, true)).To(BeTrue())
		})

		It("should ignore .log files", func() {
			logFile := filepath.Join(tempDir, "app.log")
			Expect(matcher.ShouldIgnore(tempDir, logFile, false)).To(BeTrue())
		})

		It("should ignore dist directory", func() {
			distPath := filepath.Join(tempDir, "dist")
			Expect(matcher.ShouldIgnore(tempDir, distPath, true)).To(BeTrue())
		})

		It("should ignore .env file", func() {
			envFile := filepath.Join(tempDir, ".env")
			Expect(matcher.ShouldIgnore(tempDir, envFile, false)).To(BeTrue())
		})

		It("should not ignore regular files", func() {
			regularFile := filepath.Join(tempDir, "app.js")
			Expect(matcher.ShouldIgnore(tempDir, regularFile, false)).To(BeFalse())
		})

		It("should not ignore important.log due to negation", func() {
			importantLog := filepath.Join(tempDir, "important.log")
			Expect(matcher.ShouldIgnore(tempDir, importantLog, false)).To(BeFalse())
		})
	})

	Context("when .epinioignore file does not exist", func() {
		BeforeEach(func() {
			var err error
			matcher, err = helpers.LoadIgnoreMatcher(tempDir, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an empty matcher that ignores nothing", func() {
			testFile := filepath.Join(tempDir, "any-file.txt")
			Expect(matcher.ShouldIgnore(tempDir, testFile, false)).To(BeFalse())
		})
	})

	Context("with manifest patterns", func() {
		BeforeEach(func() {
			manifestPatterns := []string{"build/", "*.tmp"}
			var err error
			matcher, err = helpers.LoadIgnoreMatcher(tempDir, manifestPatterns)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should ignore patterns from manifest", func() {
			buildPath := filepath.Join(tempDir, "build")
			Expect(matcher.ShouldIgnore(tempDir, buildPath, true)).To(BeTrue())

			tmpFile := filepath.Join(tempDir, "temp.tmp")
			Expect(matcher.ShouldIgnore(tempDir, tmpFile, false)).To(BeTrue())
		})
	})

	Context("with merged manifest and file patterns", func() {
		BeforeEach(func() {
			// Manifest patterns
			manifestPatterns := []string{"build/", "*.tmp"}

			// File patterns
			ignoreContent := `node_modules/
*.log
!important.log
`
			ignoreFile := filepath.Join(tempDir, ".epinioignore")
			err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			var err2 error
			matcher, err2 = helpers.LoadIgnoreMatcher(tempDir, manifestPatterns)
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should respect patterns from both sources", func() {
			buildPath := filepath.Join(tempDir, "build")
			Expect(matcher.ShouldIgnore(tempDir, buildPath, true)).To(BeTrue())

			nodeModulesPath := filepath.Join(tempDir, "node_modules")
			Expect(matcher.ShouldIgnore(tempDir, nodeModulesPath, true)).To(BeTrue())

			logFile := filepath.Join(tempDir, "app.log")
			Expect(matcher.ShouldIgnore(tempDir, logFile, false)).To(BeTrue())

			importantLog := filepath.Join(tempDir, "important.log")
			Expect(matcher.ShouldIgnore(tempDir, importantLog, false)).To(BeFalse())
		})
	})

	Context("with nested paths", func() {
		BeforeEach(func() {
			ignoreContent := `src/**/*.test.js
build/
`
			ignoreFile := filepath.Join(tempDir, ".epinioignore")
			err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			var err2 error
			matcher, err2 = helpers.LoadIgnoreMatcher(tempDir, nil)
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should ignore nested test files", func() {
			testFile := filepath.Join(tempDir, "src", "components", "button.test.js")
			Expect(matcher.ShouldIgnore(tempDir, testFile, false)).To(BeTrue())
		})

		It("should ignore build directory", func() {
			buildPath := filepath.Join(tempDir, "build")
			Expect(matcher.ShouldIgnore(tempDir, buildPath, true)).To(BeTrue())
		})
	})

	Context("with complex ** patterns", func() {
		BeforeEach(func() {
			ignoreContent := `a/**/b/**/c
**/test/**
src/**/*.js
`
			ignoreFile := filepath.Join(tempDir, ".epinioignore")
			err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			var err2 error
			matcher, err2 = helpers.LoadIgnoreMatcher(tempDir, nil)
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should handle multiple ** in a pattern", func() {
			filePath := filepath.Join(tempDir, "a", "x", "y", "b", "z", "w", "c")
			Expect(matcher.ShouldIgnore(tempDir, filePath, false)).To(BeTrue())
		})

		It("should handle **/pattern/**", func() {
			testFile := filepath.Join(tempDir, "any", "deep", "test", "file.txt")
			Expect(matcher.ShouldIgnore(tempDir, testFile, false)).To(BeTrue())
		})

		It("should handle pattern/**/*.ext", func() {
			jsFile := filepath.Join(tempDir, "src", "deep", "nested", "file.js")
			Expect(matcher.ShouldIgnore(tempDir, jsFile, false)).To(BeTrue())
		})
	})

	Context("with root-relative patterns", func() {
		BeforeEach(func() {
			ignoreContent := `/root-only
/subdir/
`
			ignoreFile := filepath.Join(tempDir, ".epinioignore")
			err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			var err2 error
			matcher, err2 = helpers.LoadIgnoreMatcher(tempDir, nil)
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should match root-relative patterns only at root", func() {
			rootFile := filepath.Join(tempDir, "root-only")
			Expect(matcher.ShouldIgnore(tempDir, rootFile, false)).To(BeTrue())

			nestedFile := filepath.Join(tempDir, "nested", "root-only")
			Expect(matcher.ShouldIgnore(tempDir, nestedFile, false)).To(BeFalse())
		})
	})

	Context("with negation patterns", func() {
		BeforeEach(func() {
			ignoreContent := `*.log
*.tmp
!important.log
!critical.tmp
`
			ignoreFile := filepath.Join(tempDir, ".epinioignore")
			err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			var err2 error
			matcher, err2 = helpers.LoadIgnoreMatcher(tempDir, nil)
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should ignore files matching ignore patterns", func() {
			logFile := filepath.Join(tempDir, "app.log")
			Expect(matcher.ShouldIgnore(tempDir, logFile, false)).To(BeTrue())

			tmpFile := filepath.Join(tempDir, "temp.tmp")
			Expect(matcher.ShouldIgnore(tempDir, tmpFile, false)).To(BeTrue())
		})

		It("should not ignore files matching negation patterns", func() {
			importantLog := filepath.Join(tempDir, "important.log")
			Expect(matcher.ShouldIgnore(tempDir, importantLog, false)).To(BeFalse())

			criticalTmp := filepath.Join(tempDir, "critical.tmp")
			Expect(matcher.ShouldIgnore(tempDir, criticalTmp, false)).To(BeFalse())
		})

		It("should not apply negation to files not previously ignored", func() {
			// important.txt doesn't match *.log or *.tmp, so negation shouldn't apply
			importantTxt := filepath.Join(tempDir, "important.txt")
			Expect(matcher.ShouldIgnore(tempDir, importantTxt, false)).To(BeFalse())
		})
	})

	Context("with directory-only patterns", func() {
		BeforeEach(func() {
			ignoreContent := `node_modules/
dist/
`
			ignoreFile := filepath.Join(tempDir, ".epinioignore")
			err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			var err2 error
			matcher, err2 = helpers.LoadIgnoreMatcher(tempDir, nil)
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should ignore directories but not files with same name", func() {
			nodeModulesDir := filepath.Join(tempDir, "node_modules")
			Expect(matcher.ShouldIgnore(tempDir, nodeModulesDir, true)).To(BeTrue())

			nodeModulesFile := filepath.Join(tempDir, "node_modules")
			// Create a file (not a directory) with same name - should not be ignored
			err := os.WriteFile(nodeModulesFile, []byte("test"), 0644)
			Expect(err).NotTo(HaveOccurred())
			Expect(matcher.ShouldIgnore(tempDir, nodeModulesFile, false)).To(BeFalse())
		})
	})

	Context("with unicode characters", func() {
		BeforeEach(func() {
			ignoreContent := `测试/
*.中文
`
			ignoreFile := filepath.Join(tempDir, ".epinioignore")
			err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			var err2 error
			matcher, err2 = helpers.LoadIgnoreMatcher(tempDir, nil)
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should handle unicode in patterns", func() {
			testDir := filepath.Join(tempDir, "测试")
			Expect(matcher.ShouldIgnore(tempDir, testDir, true)).To(BeTrue())

			chineseFile := filepath.Join(tempDir, "文件.中文")
			Expect(matcher.ShouldIgnore(tempDir, chineseFile, false)).To(BeTrue())
		})
	})

	Context("with very long paths", func() {
		BeforeEach(func() {
			ignoreContent := `deep/**/file.txt
`
			ignoreFile := filepath.Join(tempDir, ".epinioignore")
			err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			var err2 error
			matcher, err2 = helpers.LoadIgnoreMatcher(tempDir, nil)
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should handle very long paths", func() {
			// Create a very long path
			longPath := tempDir
			for i := 0; i < 20; i++ {
				longPath = filepath.Join(longPath, "very", "deep", "nested", "directory", "level")
			}
			longPath = filepath.Join(longPath, "file.txt")

			Expect(matcher.ShouldIgnore(tempDir, longPath, false)).To(BeTrue())
		})
	})

	Context("with pattern normalization edge cases", func() {
		BeforeEach(func() {
			ignoreContent := `  !  pattern
 pattern /
`
			ignoreFile := filepath.Join(tempDir, ".epinioignore")
			err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644)
			Expect(err).NotTo(HaveOccurred())

			var err2 error
			matcher, err2 = helpers.LoadIgnoreMatcher(tempDir, nil)
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should properly normalize patterns with whitespace", func() {
			patternFile := filepath.Join(tempDir, "pattern")
			Expect(matcher.ShouldIgnore(tempDir, patternFile, false)).To(BeFalse()) // Negation should un-ignore
		})
	})
})
