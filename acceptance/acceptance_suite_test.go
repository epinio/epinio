package acceptance_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/codeskyblue/kexec"
	"github.com/onsi/ginkgo/config"
	"github.com/suse/carrier/helpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Acceptance Suite")
}

var nodeSuffix, nodeTmpDir string
var serverURL string
var stopServerChan, diedServerChan chan bool

var _ = SynchronizedBeforeSuite(func() []byte {
	if os.Getenv("REGISTRY_USERNAME") == "" || os.Getenv("REGISTRY_PASSWORD") == "" {
		fmt.Println("REGISTRY_USERNAME or REGISTRY_PASSWORD environment variables are empty. Pulling from dockerhub will be subject to rate limiting.")
	}

	if err := checkDependencies(); err != nil {
		panic("Missing dependencies: " + err.Error())
	}

	fmt.Printf("Compiling Carrier on node %d\n", config.GinkgoConfig.ParallelNode)

	buildCarrier()

	return []byte(strconv.Itoa(int(time.Now().Unix())))
}, func(randomSuffix []byte) {
	var err error

	RegisterFailHandler(FailWithReport)

	nodeSuffix = fmt.Sprintf("%d-%s",
		config.GinkgoConfig.ParallelNode, string(randomSuffix))
	nodeTmpDir, err = ioutil.TempDir("", "carrier-"+nodeSuffix)
	if err != nil {
		panic("Could not create temp dir: " + err.Error())
	}

	copyCarrier()

	fmt.Printf("Ensuring a cluster for node %d\n", config.GinkgoConfig.ParallelNode)
	ensureCluster()
	os.Setenv("KUBECONFIG", nodeTmpDir+"/kubeconfig")

	os.Setenv("CARRIER_CONFIG", nodeTmpDir+"/carrier.yaml")

	if os.Getenv("REGISTRY_USERNAME") != "" && os.Getenv("REGISTRY_PASSWORD") != "" {
		fmt.Printf("Creating image pull secret for Dockerhub on node %d\n", config.GinkgoConfig.ParallelNode)
		helpers.Kubectl(fmt.Sprintf("create secret docker-registry regcred --docker-server=%s --docker-username=%s --docker-password=%s",
			"https://index.docker.io/v1/",
			os.Getenv("REGISTRY_USERNAME"),
			os.Getenv("REGISTRY_PASSWORD"),
		))
	}

	fmt.Printf("Installing Carrier on node %d\n", config.GinkgoConfig.ParallelNode)
	installCarrier()

	// Start Carrier server for the api tests
	serverPort := 8080 + config.GinkgoConfig.ParallelNode
	serverURL = fmt.Sprintf("http://127.0.0.1:%d", serverPort)
	serverProcess, err := startCarrierServer(serverPort)
	Expect(err).ToNot(HaveOccurred())
	go func() {
		err := serverProcess.Wait()
		if err != nil {
			diedServerChan <- true
		}
	}()

	go func() {
		for {
			select {
			case <-diedServerChan:
				// The above monitoring go routine sent us this
				panic("Carrier server died unexpectedly")
			case <-stopServerChan:
				// No panic in this case, it's the normal stop in AfterSuite.
				// This is the only part of the code that stops the server and that
				// is considered ok. The above go routine may still send true to the
				// diedServerChan but we ignore it here.
				Expect(serverProcess.Process.Kill()).ToNot(HaveOccurred())
			}
		}
	}()
})

var _ = AfterSuite(func() {
	stopServerChan <- true

	fmt.Printf("Uninstall carrier on node %d\n", config.GinkgoConfig.ParallelNode)
	out, _ := uninstallCarrier()
	match, _ := regexp.MatchString(`Carrier uninstalled`, out)
	if !match {
		panic("Uninstalling carrier failed: " + out)
	}

	fmt.Printf("Deleting tmpdir on node %d\n", config.GinkgoConfig.ParallelNode)
	deleteTmpDir()
})

func ensureCluster() {
	name := fmt.Sprintf("carrier-acceptance-%d", config.GinkgoConfig.ParallelNode)

	if _, err := exec.LookPath("k3d"); err != nil {
		panic("Couldn't find k3d in PATH: " + err.Error())
	}

	out, err := RunProc("k3d cluster get "+name, nodeTmpDir, false)
	if err != nil {
		notExists, regexpErr := regexp.Match(`No nodes found for given cluster`, []byte(out))
		if regexpErr != nil {
			panic(regexpErr)
		}
		if notExists {
			fmt.Printf("k3d cluster %s doesn't exist. I will try to create it.\n", name)
			_, err := RunProc("k3d cluster create "+name, nodeTmpDir, false)
			if err != nil {
				panic("Creating k3d cluster failed: " + err.Error())
			}
		} else {
			panic("Looking up k3d cluster failed: " + err.Error())
		}
	}

	kubeconfig, err := RunProc("k3d kubeconfig get "+name, nodeTmpDir, false)
	if err != nil {
		panic("Getting kubeconfig failed: " + err.Error())
	}
	err = ioutil.WriteFile(path.Join(nodeTmpDir, "kubeconfig"), []byte(kubeconfig), 0644)
	if err != nil {
		panic("Writing kubeconfig failed: " + err.Error())
	}
}

func deleteCluster() {
	name := fmt.Sprintf("carrier-acceptance-%s", nodeSuffix)

	if _, err := exec.LookPath("k3d"); err != nil {
		panic("Couldn't find k3d in PATH: " + err.Error())
	}

	output, err := RunProc("k3d cluster delete "+name, nodeTmpDir, false)
	if err != nil {
		panic(fmt.Sprintf("Deleting k3d cluster failed: %s\n %s\n",
			output, err.Error()))
	}
}

func deleteTmpDir() {
	err := os.RemoveAll(nodeTmpDir)
	if err != nil {
		panic(fmt.Sprintf("Failed deleting temp dir %s: %s\n",
			nodeTmpDir, err.Error()))
	}
}

func RunProc(cmd, dir string, toStdout bool) (string, error) {
	p := kexec.CommandString(cmd)

	var b bytes.Buffer
	if toStdout {
		p.Stdout = io.MultiWriter(os.Stdout, &b)
		p.Stderr = io.MultiWriter(os.Stderr, &b)
	} else {
		p.Stdout = &b
		p.Stderr = &b
	}

	p.Dir = dir

	if err := p.Run(); err != nil {
		return b.String(), err
	}

	err := p.Wait()
	return b.String(), err
}

func buildCarrier() {
	output, err := RunProc("make", "..", false)
	if err != nil {
		panic(fmt.Sprintf("Couldn't build Carrier: %s\n %s\n"+err.Error(), output))
	}
}

func copyCarrier() {
	output, err := RunProc("cp dist/carrier-* "+nodeTmpDir+"/carrier", "..", false)
	if err != nil {
		panic(fmt.Sprintf("Couldn't copy Carrier: %s\n %s\n"+err.Error(), output))
	}
}

func installCarrier() (string, error) {
	return Carrier("install", "")
}

func uninstallCarrier() (string, error) {
	return Carrier("uninstall", "")
}

// Carrier invokes the `carrier` binary, running the specified command.
// It returns the command output and/or error.
// dir parameter defines the directory from which the command should be run.
// It defaults to the current dir if left empty.
func Carrier(command string, dir string) (string, error) {
	var commandDir string
	var err error

	if dir == "" {
		commandDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	} else {
		commandDir = dir
	}

	cmd := fmt.Sprintf(nodeTmpDir+"/carrier %s", command)

	return RunProc(cmd, commandDir, false)
}

func checkDependencies() error {
	ok := true

	dependencies := []struct {
		CommandName string
	}{
		{CommandName: "wget"},
		{CommandName: "tar"},
		{CommandName: "k3d"},
	}

	for _, dependency := range dependencies {
		_, err := exec.LookPath(dependency.CommandName)
		if err != nil {
			fmt.Printf("Not found: %s\n", dependency.CommandName)
			ok = false
		}
	}

	if ok {
		return nil
	}

	return errors.New("Please check your PATH, some of our dependencies were not found")
}

func FailWithReport(message string, callerSkip ...int) {
	fmt.Println("\nA test failed. You may find the following information useful for debugging:")
	fmt.Println("The cluster pods: ")
	out, err := helpers.Kubectl("get pods --all-namespaces")
	if err != nil {
		fmt.Print(err.Error())
	} else {
		fmt.Print(out)
	}

	fmt.Println("The cluster deployments: ")
	out, err = helpers.Kubectl("get deployments --all-namespaces")
	if err != nil {
		fmt.Print(err.Error())
	} else {
		fmt.Print(out)
	}

	Fail(message, callerSkip...)
}
