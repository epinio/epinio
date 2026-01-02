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

package apps_test

import (
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/internal/cli/settings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(FailWithReport)
	RunSpecs(t, "Acceptance Suite for Application Tests")
}

var (
	nodeSuffix, nodeTmpDir string

	// serverURL is the URL of the epinio API server
	serverURL, websocketURL string

	env testenv.EpinioEnv
)

var _ = BeforeSuite(func() {
	fmt.Printf("Running tests on node %d\n", GinkgoParallelProcess())

	testenv.SetRoot("../..")
	testenv.SetupEnv()

	nodeSuffix = fmt.Sprintf("%d", GinkgoParallelProcess())
	nodeTmpDir, err := os.MkdirTemp("", "epinio-"+nodeSuffix)
	Expect(err).NotTo(HaveOccurred())

	out, err := testenv.CopyEpinioSettings(nodeTmpDir)
	Expect(err).ToNot(HaveOccurred(), out)
	os.Setenv("EPINIO_SETTINGS", nodeTmpDir+"/epinio.yaml")

	config, err := settings.LoadFrom(nodeTmpDir + "/epinio.yaml")
	Expect(err).NotTo(HaveOccurred())

	env = testenv.New(nodeTmpDir, testenv.Root(), config.User, config.Password, "", "")

	out, err = proc.Run(testenv.Root(), false, "kubectl", "get", "ingress",
		"--namespace", "epinio", "epinio",
		"-o", "jsonpath={.spec.rules[0].host}")
	Expect(err).ToNot(HaveOccurred(), out)

	// Use EPINIO_PORT environment variable if set, otherwise default to 8443
	port := os.Getenv("EPINIO_PORT")
	if port == "" {
		port = "8443"
	}
	// If port is 443, don't append it (standard HTTPS port)
	if port == "443" {
		serverURL = "https://" + out
		websocketURL = "wss://" + out
	} else {
		serverURL = "https://" + out + ":" + port
		websocketURL = "wss://" + out + ":" + port
	}
})

var _ = AfterSuite(func() {
	if !testenv.SkipCleanup() {
		fmt.Printf("Deleting tmpdir on node %d\n", GinkgoParallelProcess())
		testenv.DeleteTmpDir(nodeTmpDir)
	}
})

var _ = AfterEach(func() {
	testenv.AfterEachSleep()
})

func FailWithReport(message string, callerSkip ...int) {
	// NOTE: Use something like the following if you need to debug failed tests
	// fmt.Println("\nA test failed. You may find the following information useful for debugging:")
	// fmt.Println("The cluster pods: ")
	// out, err := proc.Kubectl("get pods --all-namespaces")
	// if err != nil {
	// 	fmt.Print(err.Error())
	// } else {
	// 	fmt.Print(out)
	// }

	// Ensures the correct line numbers are reported
	Fail(message, callerSkip[0]+1)
}

// getPortSuffixFromServerURL extracts the port suffix (with colon prefix) from serverURL.
// Returns the port with a colon prefix, e.g., ":8443" from "https://example.com:8443".
// Returns empty string for default HTTPS port (443) or if no port is specified.
// Falls back to ":8443" if parsing fails.
func getPortSuffixFromServerURL() string {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		// If parsing fails, return default port
		return ":8443"
	}

	port := parsed.Port()
	if port == "" {
		// No port specified - for HTTPS, default is 443, return empty string
		// (routes will work without explicit port for standard HTTPS)
		return ""
	}

	// If port is 443, return empty string (standard HTTPS, no need to append)
	if port == "443" {
		return ""
	}

	return ":" + port
}

