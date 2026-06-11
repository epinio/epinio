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

package acceptance_test

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Apps watch", LApplication, func() {

	var (
		namespace string
		appName   string
		workDir   string
		route     string
		watchCmd  *exec.Cmd
	)

	// copyAsset copies the named asset file from the golang sample app into
	// the temporary working directory.
	copyAsset := func(name string) {
		sourcePath := testenv.AssetPath("golang-sample-app", name)
		content, readError := os.ReadFile(sourcePath)
		Expect(readError).ToNot(HaveOccurred())
		writeError := os.WriteFile(
			filepath.Join(workDir, name), content, 0o644,
		)
		Expect(writeError).ToNot(HaveOccurred())
	}

	curlApp := func() string {
		response, curlError := env.Curl("GET", route, strings.NewReader(""))
		if curlError != nil {
			return ""
		}
		defer func() {
			_ = response.Body.Close()
		}()
		if response.StatusCode != http.StatusOK {
			return ""
		}
		content, readError := io.ReadAll(response.Body)
		if readError != nil {
			return ""
		}
		return string(content)
	}

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()

		// Work on a throwaway copy so the watch state file, sync config,
		// build output, and source edits never touch the repo checkout.
		workDir = GinkgoT().TempDir()
		copyAsset("main.go")
		copyAsset("go.mod")
		copyAsset("go.sum")

		writeGitignoreError := os.WriteFile(
			filepath.Join(workDir, ".gitignore"),
			[]byte("bin/\n"),
			0o644,
		)
		Expect(writeGitignoreError).ToNot(HaveOccurred())

		writeConfigError := os.WriteFile(
			filepath.Join(workDir, ".epinio-sync.yaml"),
			[]byte(
				"build_cmd: \"CGO_ENABLED=0 go build -o ./bin/app .\"\n"+
					"binary: \"./bin/app\"\n",
			),
			0o644,
		)
		Expect(writeConfigError).ToNot(HaveOccurred())

		pushOutput, pushError := env.EpinioPush(
			workDir, appName, "--name", appName,
		)
		Expect(pushError).ToNot(HaveOccurred(), pushOutput)

		route = testenv.AppRouteWithPort(
			testenv.AppRouteFromOutput(pushOutput),
		)
		Expect(route).ToNot(BeEmpty(), pushOutput)
	})

	AfterEach(func() {
		if watchCmd != nil && watchCmd.Process != nil {
			killError := watchCmd.Process.Kill()
			if killError != nil {
				GinkgoWriter.Printf(
					"failed to kill watch process: %v\n", killError,
				)
			}
			_ = watchCmd.Wait()
		}
		env.DeleteApp(appName)
		env.DeleteNamespace(namespace)
	})

	It("starts up, then syncs a binary change into the pod", func() {
		watchOutput := gbytes.NewBuffer()

		watchCmd = env.EpinioCmd("apps", "watch", appName)
		watchCmd.Dir = workDir
		watchCmd.Stdout = io.MultiWriter(watchOutput, GinkgoWriter)
		watchCmd.Stderr = io.MultiWriter(watchOutput, GinkgoWriter)

		startError := watchCmd.Start()
		Expect(startError).ToNot(HaveOccurred())

		By("waiting for the startup patch to complete")
		Eventually(watchOutput, "10m").Should(
			gbytes.Say("Watching for changes"),
		)

		By("verifying the app serves the original content")
		Eventually(curlApp, "2m", "2s").Should(
			ContainSubstring("Paketo Buildpacks"),
		)

		By("changing the handler response in the source")
		mainPath := filepath.Join(workDir, "main.go")
		source, readError := os.ReadFile(mainPath)
		Expect(readError).ToNot(HaveOccurred())

		patched := strings.Replace(
			string(source),
			"Powered By Paketo Buildpacks",
			"EPINIO-WATCH-SYNC-OK",
			1,
		)
		Expect(patched).ToNot(Equal(string(source)))

		writeError := os.WriteFile(mainPath, []byte(patched), 0o644)
		Expect(writeError).ToNot(HaveOccurred())

		By("waiting for the watcher to build and sync")
		Eventually(watchOutput, "3m").Should(gbytes.Say("Synced in"))

		By("verifying the app serves the changed content")
		Eventually(curlApp, "2m", "2s").Should(
			ContainSubstring("EPINIO-WATCH-SYNC-OK"),
		)
	})
})
