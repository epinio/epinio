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
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/auth"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/internal/cli/settings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(FailWithReport)
	RunSpecs(t, "Acceptance Suite")
}

var (
	// Labels for test sections.
	LService       = Label("service")
	LAppchart      = Label("appchart")
	LApplication   = Label("application")
	LConfiguration = Label("configuration")
	LNamespace     = Label("namespace")
	LGitconfig     = Label("gitconfig")
	LMisc          = Label("misc")

	// Test configuration and state
	nodeSuffix, nodeTmpDir  string
	serverURL, websocketURL string

	env testenv.EpinioEnv
	r   *rand.Rand
)

// BeforeSuiteMessage is a serializable struct that can be passed through the SynchronizedBeforeSuite
type BeforeSuiteMessage struct {
	AdminToken string `json:"admin_token"`
	UserToken  string `json:"user_token"`
}

var _ = SynchronizedBeforeSuite(func() []byte {
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
	r = rand.New(rand.NewSource(time.Now().UnixNano()))

	var message BeforeSuiteMessage
	err := json.Unmarshal(msg, &message)
	Expect(err).NotTo(HaveOccurred())

	fmt.Printf("Running tests on node %d\n", GinkgoParallelProcess())

	testenv.SetRoot("..")
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

	serverURL = "https://" + out
	websocketURL = "wss://" + out
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
