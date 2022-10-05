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

	// Lets see if ok with init
	env testenv.EpinioEnv
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
		"--wait",
	)
	Expect(err).NotTo(HaveOccurred(), out)
}

func InstallNginx() {
	out, err := proc.RunW("helm", "repo", "add", "nginx-stable", "https://helm.nginx.com/stable")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "repo", "update")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "upgrade", "--install", "nginx-ingress", "nginx-stable/nginx-ingress",
		"-n", "ingress-nginx",
		"--create-namespace",
		"--set", "controller.setAsDefaultIngress=true",
		"--set", "controller.service.name=ingress-nginx-controller",
	)
	Expect(err).NotTo(HaveOccurred(), out)
}

func InstallTraefik() {
	out, err := proc.RunW("helm", "repo", "add", "traefik", "https://helm.traefik.io/traefik")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "repo", "update")
	Expect(err).NotTo(HaveOccurred(), out)
	out, err = proc.RunW("helm", "upgrade", "--install", "traefik", "traefik/traefik",
		"-n", "traefik",
		"--create-namespace",
		"--set", "ports.web.redirectTo=websecure",
		"--set", "ingressClass.enabled=true",
		"--set", "ingressClass.isDefaultClass=true",
	)
	Expect(err).NotTo(HaveOccurred(), out)
}

var _ = SynchronizedBeforeSuite(func() []byte {
	ingressController := os.Getenv("INGRESS_CONTROLLER")

	By("Installing and configuring the prerequisites", func() {
		testenv.SetRoot("../..")
		testenv.SetupEnv()

		env = testenv.New(nodeTmpDir, testenv.Root(), "admin", "password")
	})

	released := os.Getenv("EPINIO_RELEASED")
	isreleased := released == "true"
	if !isreleased {
		By("Compiling Epinio", func() {
			testenv.BuildEpinio()
		})
	} else {
		By("Expecting a client binary")
	}

	By("Creating registry secret", func() {
		testenv.CreateRegistrySecret()
	})

	By("Installing cert-manager", func() {
		InstallCertManager()
	})

	By("Installing ingress controller", func() {
		if ingressController == "nginx" {
			fmt.Printf("Using nginx\n")
			InstallNginx()
		} else if ingressController == "traefik" {
			fmt.Printf("Using traefik\n")
			InstallTraefik()
		}
	})

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
