package install_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/acceptance/testenv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(FailWithReport)
	RunSpecs(t, "Acceptance Suite")
}

var (
	nodeTmpDir string
)

func InstallCertManager() {
	out, err := proc.RunW("helm", "repo", "add", "jetstack", "https://charts.jetstack.io")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "repo", "update")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "upgrade", "--install", "cert-manager", "jetstack/cert-manager",
		"-n", "cert-manager",
		"--create-namespace",
		"--set", "installCRDs=true",
		"--set", "extraArgs[0]=--enable-certificate-owner-ref=true",
	)
	Expect(err).NotTo(HaveOccurred(), out)
}

var _ = BeforeSuite(func() {
	InstallCertManager()
})

var _ = SynchronizedBeforeSuite(func() []byte {
	fmt.Printf("I'm running on runner = %s\n", os.Getenv("HOSTNAME"))

	testenv.SetRoot("../..")
	testenv.SetupEnv()

	fmt.Printf("Compiling Epinio on node %d\n", GinkgoParallelProcess())
	testenv.BuildEpinio()

	testenv.CreateRegistrySecret()

	return []byte{}
}, func(_ []byte) {
	testenv.SetRoot("../..")
	testenv.SetupEnv()

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
	// out, err := proc.Kubectl("get pods --all-namespaces")
	// if err != nil {
	// 	fmt.Print(err.Error())
	// } else {
	// 	fmt.Print(out)
	// }

	// Ensures the correct line numbers are reported
	Fail(message, callerSkip[0]+1)
}
