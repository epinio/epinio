package v1_test

import (
	"encoding/json"
	"fmt"
	"io"
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
