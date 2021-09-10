package apps_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"
	"github.com/epinio/epinio/helpers"
	"github.com/onsi/ginkgo/config"

	. "github.com/onsi/ginkgo"
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

var _ = SynchronizedBeforeSuite(func() []byte {
	// Singleton setup. Run on node 1 before all

	fmt.Printf("I'm running on runner = %s\n", os.Getenv("HOSTNAME"))

	testenv.SetRoot("../..")

	testenv.SetupEnv()

	if err := testenv.CheckDependencies(); err != nil {
		panic("Missing dependencies: " + err.Error())
	}

	fmt.Printf("Compiling Epinio on node %d\n", config.GinkgoConfig.ParallelNode)
	testenv.BuildEpinio()

	testenv.CreateRegistrySecret()

	epinioBinary := testenv.EpinioBinaryPath()
	err := testenv.CheckEpinio()
	Expect(err).ToNot(HaveOccurred(), "installing epinio")

	out, err := testenv.PatchEpinio()
	Expect(err).ToNot(HaveOccurred(), out)

	// Now create the default org which we skipped because it would fail before
	// patching.
	// NOTE: Unfortunately this prevents us from testing if the `install` command
	// really creates a default workspace. Needs a better solution that allows
	// install to do it's thing without needing the patch script to run first.
	// Eventually is used to retry in case the rollout of the patched deployment
	// is not completely done yet.
	fmt.Println("Ensure default workspace exists")
	testenv.EnsureDefaultWorkspace(epinioBinary)

	fmt.Println("Setup cluster services")
	testenv.SetupInClusterServices(epinioBinary)

	out, err = helpers.Kubectl("get", "pods", "--namespace", "minibroker", "--selector", "app=minibroker-minibroker")
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).To(MatchRegexp(`minibroker.*2/2.*Running`))

	fmt.Println("Setup google")
	err = testenv.SetupGoogleServices(epinioBinary)
	Expect(err).ToNot(HaveOccurred(), out)

	fmt.Println("SynchronizedBeforeSuite is done, checking Epinio info endpoint")
	testenv.ExpectGoodInstallation(epinioBinary)

	return []byte(strconv.Itoa(int(time.Now().Unix())))
}, func(randomSuffix []byte) {
	var err error
	testenv.SetRoot("../..")

	nodeSuffix = fmt.Sprintf("%d-%s",
		config.GinkgoConfig.ParallelNode, string(randomSuffix))
	nodeTmpDir, err = ioutil.TempDir("", "epinio-"+nodeSuffix)
	if err != nil {
		panic("Could not create temp dir: " + err.Error())
	}

	Expect(os.Getenv("KUBECONFIG")).ToNot(BeEmpty(), "KUBECONFIG environment variable should not be empty")

	// Get config from the installation (API credentials)
	out, err := testenv.CopyEpinioConfig(nodeTmpDir)
	Expect(err).ToNot(HaveOccurred(), out)
	os.Setenv("EPINIO_CONFIG", nodeTmpDir+"/epinio.yaml")

	env = testenv.New(nodeTmpDir, testenv.Root())

	out, err = env.Epinio(nodeTmpDir, "target", "workspace")
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = proc.Run(testenv.Root(), false, "kubectl", "get", "ingress",
		"--namespace", "epinio", "epinio",
		"-o", "jsonpath={.spec.rules[0].host}")
	Expect(err).ToNot(HaveOccurred(), out)

	serverURL = "https://" + out
	websocketURL = "wss://" + out
})

var _ = SynchronizedAfterSuite(func() {
	if !testenv.SkipCleanup() {
		fmt.Printf("Deleting tmpdir on node %d\n", config.GinkgoConfig.ParallelNode)
		testenv.DeleteTmpDir(nodeTmpDir)
	}
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
