package v1_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	epinioConfig "github.com/epinio/epinio/internal/cli/config"

	"github.com/onsi/ginkgo/config"

	. "github.com/onsi/ginkgo"
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

var _ = SynchronizedBeforeSuite(func() []byte {
	fmt.Println("Creating the minio helper pod")
	createS3HelperPod()

	return []byte{}
}, func(_ []byte) {
	fmt.Printf("Running tests on node %d\n", config.GinkgoConfig.ParallelNode)

	testenv.SetRoot("../../..")
	testenv.SetupEnv()

	nodeSuffix = fmt.Sprintf("%d", config.GinkgoConfig.ParallelNode)
	nodeTmpDir, err := ioutil.TempDir("", "epinio-"+nodeSuffix)
	Expect(err).NotTo(HaveOccurred())

	out, err := testenv.CopyEpinioConfig(nodeTmpDir)
	Expect(err).ToNot(HaveOccurred(), out)
	os.Setenv("EPINIO_CONFIG", nodeTmpDir+"/epinio.yaml")

	config, err := epinioConfig.LoadFrom(nodeTmpDir + "/epinio.yaml")
	Expect(err).NotTo(HaveOccurred())
	env = testenv.New(nodeTmpDir, testenv.Root(), config.User, config.Password)

	out, err = proc.Run(testenv.Root(), false, "kubectl", "get", "ingress",
		"--namespace", "epinio", "epinio",
		"-o", "jsonpath={.spec.rules[0].host}")
	Expect(err).ToNot(HaveOccurred(), out)

	serverURL = "https://" + out
	websocketURL = "wss://" + out
})

var _ = AfterSuite(func() {
	if !testenv.SkipCleanup() {
		fmt.Printf("Deleting tmpdir on node %d\n", config.GinkgoConfig.ParallelNode)
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
	// out, err := helpers.Kubectl("get pods --all-namespaces")
	// if err != nil {
	// 	fmt.Print(err.Error())
	// } else {
	// 	fmt.Print(out)
	// }

	// Ensures the correct line numbers are reported
	Fail(message, callerSkip[0]+1)
}
