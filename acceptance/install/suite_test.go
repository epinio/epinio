package install_test

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/onsi/ginkgo/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(FailWithReport)
	RunSpecs(t, "Acceptance Suite")
}

var (
	nodeSuffix, nodeTmpDir string
	env                    testenv.EpinioEnv
)

var _ = SynchronizedBeforeSuite(func() []byte {
	fmt.Printf("I'm running on runner = %s\n", os.Getenv("HOSTNAME"))

	testenv.SetRoot("../..")
	testenv.SetupEnv()

	if err := testenv.CheckDependencies(); err != nil {
		panic("Missing dependencies: " + err.Error())
	}

	fmt.Printf("Compiling Epinio on node %d\n", config.GinkgoConfig.ParallelNode)
	testenv.BuildEpinio()

	testenv.CreateRegistrySecret()

	return []byte(strconv.Itoa(int(time.Now().Unix())))
}, func(randomSuffix []byte) {
	testenv.SetRoot("../..")
	os.Setenv("EPINIO_CONFIG", testenv.EpinioYAML())

	Expect(os.Getenv("KUBECONFIG")).ToNot(BeEmpty(), "KUBECONFIG environment variable should not be empty")
})

var _ = SynchronizedAfterSuite(func() {
}, func() { // Runs only on one node after all are done
	if testenv.SkipCleanup() {
		fmt.Printf("Found '%s', skipping all cleanup", testenv.SkipCleanupPath())
	} else {
		// Delete left-overs no matter what
		defer func() { _, _ = testenv.CleanupTmp() }()
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
