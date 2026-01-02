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

package v1_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/epinio/epinio/acceptance/helpers/auth"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(FailWithReport)
	RunSpecs(t, "APIv1 Suite")
}

var (
	// Labels for test sections.
	LService       = Label("service")
	LAppchart      = Label("appchart")
	LApplication   = Label("application")
	LConfiguration = Label("configuration")
	LNamespace     = Label("namespace")
	LMisc          = Label("misc")

	// Test configuration and state
	nodeSuffix, nodeTmpDir  string
	serverURL, websocketURL string

	env testenv.EpinioEnv
)

// BeforeSuiteMessage is a serializable struct that can be passed through the SynchronizedBeforeSuite
type BeforeSuiteMessage struct {
	AdminToken string `json:"admin_token"`
	UserToken  string `json:"user_token"`
}

var _ = SynchronizedBeforeSuite(func() []byte {
	fmt.Println("Creating the minio helper pod")
	createS3HelperPod()

	// login just once
	globalSettings, err := settings.LoadFrom(testenv.EpinioYAML())
	Expect(err).NotTo(HaveOccurred())

	adminToken, err := auth.GetToken(globalSettings.API, "admin@epinio.io", "password")
	Expect(err).NotTo(HaveOccurred())
	userToken, err := auth.GetToken(globalSettings.API, "epinio@epinio.io", "password")
	Expect(err).NotTo(HaveOccurred())

	msg, err := json.Marshal(BeforeSuiteMessage{
		AdminToken: adminToken,
		UserToken:  userToken,
	})
	Expect(err).NotTo(HaveOccurred())

	return msg
}, func(msg []byte) {
	var message BeforeSuiteMessage
	err := json.Unmarshal(msg, &message)
	Expect(err).NotTo(HaveOccurred())

	fmt.Printf("Running tests on node %d\n", GinkgoParallelProcess())

	testenv.SetRoot("../../..")
	testenv.SetupEnv()

	nodeSuffix = fmt.Sprintf("%d", GinkgoParallelProcess())
	nodeTmpDir, err := os.MkdirTemp("", "epinio-"+nodeSuffix)
	Expect(err).NotTo(HaveOccurred())

	out, err := testenv.CopyEpinioSettings(nodeTmpDir)
	Expect(err).ToNot(HaveOccurred(), out)
	os.Setenv("EPINIO_SETTINGS", nodeTmpDir+"/epinio.yaml")

	theSettings, err := settings.LoadFrom(nodeTmpDir + "/epinio.yaml")
	Expect(err).NotTo(HaveOccurred())

	env = testenv.New(nodeTmpDir, testenv.Root(), theSettings.User, theSettings.Password, message.AdminToken, message.UserToken)

	out, err = proc.Run(testenv.Root(), false, "kubectl", "get", "ingress",
		"--namespace", "epinio", "epinio",
		"-o", "jsonpath={.spec.rules[0].host}")
	Expect(err).ToNot(HaveOccurred(), out)

	serverURL = "https://" + out + ":8443"
	websocketURL = "wss://" + out + ":8443"
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

func authToken() (string, error) {
	authURL := fmt.Sprintf("%s%s/%s", serverURL, v1.Root, v1.Routes.Path("AuthToken"))
	response, err := env.Curl("GET", authURL, strings.NewReader(""))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	tr := &models.AuthTokenResponse{}
	err = json.Unmarshal(bodyBytes, &tr)

	return tr.Token, err
}

// getPortSuffixFromServerURL extracts the port suffix (with colon prefix) from serverURL.
// Returns the port with a colon prefix, e.g., ":8443" from "https://example.com:8443".
// Falls back to ":8443" if parsing fails or no port is found.
func getPortSuffixFromServerURL() string {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		// If parsing fails, return default port
		return ":8443"
	}

	port := parsed.Port()
	if port == "" {
		// No port specified, return default
		return ":8443"
	}

	// Validate that port is numeric (basic check)
	// Port should be a number, but we'll be lenient and just return it with colon
	// The actual HTTP request will fail if the port is invalid
	return ":" + port
}
