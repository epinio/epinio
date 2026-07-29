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
	"archive/tar"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("watch helpers", func() {

	var workDir string

	BeforeEach(func() {
		var makeDirError error
		workDir, makeDirError = os.MkdirTemp("", "watch-test-*")
		Expect(makeDirError).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		removeDirError := os.RemoveAll(workDir)
		Expect(removeDirError).ToNot(HaveOccurred())
	})

	writeFile := func(relPath, content string) {
		fullPath := filepath.Join(workDir, relPath)
		makeDirError := os.MkdirAll(filepath.Dir(fullPath), 0o755)
		Expect(makeDirError).ToNot(HaveOccurred())
		writeError := os.WriteFile(fullPath, []byte(content), 0o644)
		Expect(writeError).ToNot(HaveOccurred())
	}

	Describe("readIgnoreFile", func() {

		It("strips comments and blank lines", func() {
			writeFile(".gitignore", "# a comment\n\nbin/\n  \n*.log\n")

			patterns, readError := readIgnoreFile(
				filepath.Join(workDir, ".gitignore"),
			)
			Expect(readError).ToNot(HaveOccurred())
			Expect(patterns).To(Equal([]string{"bin/", "*.log"}))
		})

		It("returns nothing for a missing file", func() {
			patterns, readError := readIgnoreFile(
				filepath.Join(workDir, "does-not-exist"),
			)
			Expect(readError).ToNot(HaveOccurred())
			Expect(patterns).To(BeNil())
		})
	})

	Describe("md5File", func() {

		It("returns the md5 hex digest of the file content", func() {
			writeFile("data.txt", "hello world")

			digest, hashError := md5File(filepath.Join(workDir, "data.txt"))
			Expect(hashError).ToNot(HaveOccurred())
			Expect(digest).To(Equal("5eb63bbbe01eeed093cb22bb8f5acdc3"))
		})

		It("fails for a missing file", func() {
			_, hashError := md5File(filepath.Join(workDir, "nope"))
			Expect(hashError).To(HaveOccurred())
		})
	})

	Describe("loadSyncConfig", func() {

		It("parses all fields", func() {
			writeFile(watchConfigFile,
				"build_cmd: \"go build -o ./bin/app .\"\n"+
					"binary: \"./bin/app\"\n"+
					"files_dest: \"/srv/app\"\n"+
					"binary_dest: \"/srv/bin/app\"\n"+
					"process_cmd: \"/srv/bin/start\"\n",
			)

			cfg, loadError := loadSyncConfig(workDir)
			Expect(loadError).ToNot(HaveOccurred())
			Expect(cfg.BuildCmd).To(Equal("go build -o ./bin/app ."))
			Expect(cfg.Binary).To(Equal("./bin/app"))
			Expect(cfg.FilesDest).To(Equal("/srv/app"))
			Expect(cfg.BinaryDest).To(Equal("/srv/bin/app"))
			Expect(cfg.ProcessCmd).To(Equal("/srv/bin/start"))
		})

		It("returns an empty config when the file is missing", func() {
			cfg, loadError := loadSyncConfig(workDir)
			Expect(loadError).ToNot(HaveOccurred())
			Expect(cfg).To(Equal(syncConfig{}))
		})

		It("fails on malformed yaml", func() {
			writeFile(watchConfigFile, "build_cmd: [not a string\n")

			_, loadError := loadSyncConfig(workDir)
			Expect(loadError).To(HaveOccurred())
		})
	})

	Describe("saveHashes and loadHashes", func() {

		It("round-trips a hash map", func() {
			statePath := filepath.Join(workDir, watchStateFile)
			saved := fileHashes{
				"main.go":     "abc123",
				"sub/util.go": "def456",
			}

			saveError := saveHashes(statePath, saved)
			Expect(saveError).ToNot(HaveOccurred())

			loaded, loadError := loadHashes(statePath)
			Expect(loadError).ToNot(HaveOccurred())
			Expect(loaded).To(Equal(saved))
		})

		It("returns nil for a missing state file", func() {
			loaded, loadError := loadHashes(
				filepath.Join(workDir, watchStateFile),
			)
			Expect(loadError).ToNot(HaveOccurred())
			Expect(loaded).To(BeNil())
		})

		It("fails on a corrupt state file", func() {
			writeFile(watchStateFile, "not json")

			_, loadError := loadHashes(filepath.Join(workDir, watchStateFile))
			Expect(loadError).To(HaveOccurred())
		})
	})

	Describe("diffHashes", func() {

		It("detects changed, new, and deleted files", func() {
			old := fileHashes{
				"unchanged.go": "aaa",
				"changed.go":   "bbb",
				"deleted.go":   "ccc",
			}
			current := fileHashes{
				"unchanged.go": "aaa",
				"changed.go":   "BBB",
				"new.go":       "ddd",
			}

			changed, deleted := diffHashes(old, current)
			Expect(changed).To(ConsistOf("changed.go", "new.go"))
			Expect(deleted).To(ConsistOf("deleted.go"))
		})

		It("returns nothing when both sides match", func() {
			hashes := fileHashes{"main.go": "aaa"}

			changed, deleted := diffHashes(hashes, hashes)
			Expect(changed).To(BeEmpty())
			Expect(deleted).To(BeEmpty())
		})
	})

	Describe("hashDir", func() {

		It("hashes regular files including nested directories", func() {
			writeFile("main.go", "package main")
			writeFile("sub/util.go", "package sub")

			hashes, hashError := hashDir(workDir)
			Expect(hashError).ToNot(HaveOccurred())
			Expect(hashes).To(HaveLen(2))
			Expect(hashes).To(HaveKey("main.go"))
			Expect(hashes).To(HaveKey(filepath.Join("sub", "util.go")))
		})

		It("respects .gitignore patterns", func() {
			writeFile(".gitignore", "bin/\n*.log\n")
			writeFile("main.go", "package main")
			writeFile("bin/app", "binary")
			writeFile("debug.log", "noise")

			hashes, hashError := hashDir(workDir)
			Expect(hashError).ToNot(HaveOccurred())
			Expect(hashes).To(HaveKey("main.go"))
			Expect(hashes).To(HaveKey(".gitignore"))
			Expect(hashes).ToNot(HaveKey(filepath.Join("bin", "app")))
			Expect(hashes).ToNot(HaveKey("debug.log"))
		})

		It("respects .epinioignore patterns", func() {
			writeFile(".epinioignore", "fixtures/\n")
			writeFile("main.go", "package main")
			writeFile("fixtures/big.dat", "data")

			hashes, hashError := hashDir(workDir)
			Expect(hashError).ToNot(HaveOccurred())
			Expect(hashes).To(HaveKey("main.go"))
			Expect(hashes).ToNot(HaveKey(filepath.Join("fixtures", "big.dat")))
		})

		It("always skips .git and the watch state and config files", func() {
			writeFile("main.go", "package main")
			writeFile(".git/HEAD", "ref: refs/heads/main")
			writeFile(watchStateFile, "{}")
			writeFile(watchConfigFile, "binary: ./bin/app")

			hashes, hashError := hashDir(workDir)
			Expect(hashError).ToNot(HaveOccurred())
			Expect(hashes).To(HaveLen(1))
			Expect(hashes).To(HaveKey("main.go"))
		})

		It("returns an empty map for an empty directory", func() {
			hashes, hashError := hashDir(workDir)
			Expect(hashError).ToNot(HaveOccurred())
			Expect(hashes).To(BeEmpty())
		})
	})

	Describe("createSyncTar", func() {

		readTarEntries := func(path string) map[string]string {
			tarFile, openError := os.Open(path)
			Expect(openError).ToNot(HaveOccurred())
			defer func() {
				closeError := tarFile.Close()
				Expect(closeError).ToNot(HaveOccurred())
			}()

			entries := map[string]string{}
			tarReader := tar.NewReader(tarFile)
			for {
				header, nextError := tarReader.Next()
				if nextError == io.EOF {
					break
				}
				Expect(nextError).ToNot(HaveOccurred())
				content, readError := io.ReadAll(tarReader)
				Expect(readError).ToNot(HaveOccurred())
				entries[header.Name] = string(content)
			}
			return entries
		}

		It("contains exactly the listed files with their contents", func() {
			writeFile("main.go", "package main")
			writeFile("sub/util.go", "package sub")
			writeFile("skipped.go", "not included")
			tarPath := filepath.Join(workDir, "sync.tar")

			createError := createSyncTar(
				tarPath,
				workDir,
				[]string{"main.go", filepath.Join("sub", "util.go")},
			)
			Expect(createError).ToNot(HaveOccurred())

			entries := readTarEntries(tarPath)
			Expect(entries).To(HaveLen(2))
			Expect(entries["main.go"]).To(Equal("package main"))
			Expect(entries[filepath.Join("sub", "util.go")]).To(
				Equal("package sub"),
			)
		})

		It("fails when a listed file is missing", func() {
			tarPath := filepath.Join(workDir, "sync.tar")

			createError := createSyncTar(tarPath, workDir, []string{"ghost.go"})
			Expect(createError).To(HaveOccurred())
			Expect(createError.Error()).To(ContainSubstring("ghost.go"))
		})
	})

	Describe("runBuildCmd", func() {

		It("runs the command in the given working directory", func() {
			runError := runBuildCmd("echo built > out.txt", workDir)
			Expect(runError).ToNot(HaveOccurred())
			Expect(filepath.Join(workDir, "out.txt")).To(BeAnExistingFile())
		})

		It("fails when the command fails", func() {
			runError := runBuildCmd("exit 1", workDir)
			Expect(runError).To(HaveOccurred())
		})
	})
})
