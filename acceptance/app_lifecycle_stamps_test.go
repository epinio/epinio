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

package acceptance_test

import (
	"os"
	"path/filepath"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Lifecycle timing stamps (EPINIO-783): status.lastbuild and status.lastdeploy
// on the App CR, written by the build and deploy paths.
var _ = Describe("App lifecycle timing stamps", LApplication, func() {
	var (
		namespace string
		appName   string
	)

	// jsonpath reads one field from the App CR status or spec.
	jsonpath := func(field string) string {
		out, err := proc.Kubectl("get", "app",
			"--namespace", namespace, appName,
			"-o", "jsonpath={"+field+"}")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		appName = catalog.NewAppName()
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	It("stamps lastbuild and lastdeploy as succeeded after a successful push", func() {
		out, err := env.EpinioPush("../assets/sample-app", appName, "--name", appName)
		Expect(err).ToNot(HaveOccurred(), out)

		Expect(jsonpath(".spec.buildstatus")).To(Equal("deployed"))

		Expect(jsonpath(".status.lastbuild.result")).To(Equal("succeeded"))
		Expect(jsonpath(".status.lastbuild.durationms")).ToNot(Equal("0"))
		Expect(jsonpath(".status.lastbuild.finishedat")).ToNot(BeEmpty())
		Expect(jsonpath(".status.lastbuild.stageid")).ToNot(BeEmpty())

		Expect(jsonpath(".status.lastdeploy.result")).To(Equal("succeeded"))
		Expect(jsonpath(".status.lastdeploy.durationms")).ToNot(Equal("0"))
		Expect(jsonpath(".status.lastdeploy.finishedat")).ToNot(BeEmpty())
	})

	It("moves lastbuild and lastdeploy forward on a redeploy", func() {
		out, err := env.EpinioPush("../assets/sample-app", appName, "--name", appName)
		Expect(err).ToNot(HaveOccurred(), out)

		firstBuild := jsonpath(".status.lastbuild.finishedat")
		firstDeploy := jsonpath(".status.lastdeploy.finishedat")

		out, err = env.EpinioPush("../assets/sample-app", appName, "--name", appName)
		Expect(err).ToNot(HaveOccurred(), out)

		// finishedat has one-second resolution, so a redeploy that starts
		// and ends in the same second would not move it. Sample app builds
		// take longer than that.
		Expect(jsonpath(".status.lastbuild.finishedat")).ToNot(Equal(firstBuild))
		Expect(jsonpath(".status.lastdeploy.finishedat")).ToNot(Equal(firstDeploy))
	})

	It("stamps lastbuild as failed when the build fails", func() {
		// A package.json that depends on a package that does not exist makes
		// `npm install` fail inside the buildpack, so the build fails.
		brokenDir, err := os.MkdirTemp("", "broken-app-")
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(os.RemoveAll, brokenDir)

		Expect(os.WriteFile(filepath.Join(brokenDir, "server.js"),
			[]byte(`console.log("hi")`), 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(brokenDir, "package.json"), []byte(`{
  "name": "broken",
  "version": "1.0.0",
  "main": "server.js",
  "dependencies": {
    "epinio-nonexistent-pkg-783": "1.0.0"
  }
}`), 0o600)).To(Succeed())

		out, err := env.EpinioPush(brokenDir, appName, "--name", appName)
		Expect(err).To(HaveOccurred(), out)

		Expect(jsonpath(".status.lastbuild.result")).To(Equal("failed"))
		Expect(jsonpath(".status.lastbuild.durationms")).ToNot(Equal("0"))
	})

	// Deploy failure is not covered yet. A route conflict is rejected by the
	// deploy endpoint before the deploy step runs, so it does not reach the
	// lastdeploy stamp. A reliable trigger needs a deploy-time failure (for
	// example a cluster ResourceQuota), which depends on the test cluster.
	PIt("stamps lastdeploy as failed when the deploy step fails", func() {})
})
