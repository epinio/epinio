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
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/codeskyblue/kexec"
	"github.com/epinio/epinio/helpers"
	"github.com/onsi/ginkgo/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Acceptance Suite")
}

var nodeSuffix, nodeTmpDir string
var serverURL string

var registryMirrorName = "epinio-acceptance-registry-mirror"

const (
	networkName       = "epinio-acceptance"
	registryMirrorEnv = "EPINIO_REGISTRY_CONFIG"
)

var _ = SynchronizedBeforeSuite(func() []byte {
	if os.Getenv("REGISTRY_USERNAME") == "" || os.Getenv("REGISTRY_PASSWORD") == "" {
		fmt.Println("REGISTRY_USERNAME or REGISTRY_PASSWORD environment variables are empty. Pulling from dockerhub will be subject to rate limiting.")
	}

	if err := checkDependencies(); err != nil {
		panic("Missing dependencies: " + err.Error())
	}

	fmt.Printf("Compiling Epinio on node %d\n", config.GinkgoConfig.ParallelNode)

	buildEpinio()
	fmt.Println("Ensuring a docker network")
	out, err := ensureRegistryNetwork()
	Expect(err).ToNot(HaveOccurred(), out)
	out, err = ensureRegistryMirror()
	Expect(err).ToNot(HaveOccurred(), out)

	return []byte(strconv.Itoa(int(time.Now().Unix())))
}, func(randomSuffix []byte) {
	var err error

	RegisterFailHandler(FailWithReport)

	nodeSuffix = fmt.Sprintf("%d-%s",
		config.GinkgoConfig.ParallelNode, string(randomSuffix))
	nodeTmpDir, err = ioutil.TempDir("", "epinio-"+nodeSuffix)
	if err != nil {
		panic("Could not create temp dir: " + err.Error())
	}

	copyEpinio()

	fmt.Printf("Ensuring a cluster for node %d\n", config.GinkgoConfig.ParallelNode)
	ensureCluster()
	os.Setenv("KUBECONFIG", nodeTmpDir+"/kubeconfig")

	os.Setenv("EPINIO_CONFIG", nodeTmpDir+"/epinio.yaml")

	if os.Getenv("REGISTRY_USERNAME") != "" && os.Getenv("REGISTRY_PASSWORD") != "" {
		fmt.Printf("Creating image pull secret for Dockerhub on node %d\n", config.GinkgoConfig.ParallelNode)
		helpers.Kubectl(fmt.Sprintf("create secret docker-registry regcred --docker-server=%s --docker-username=%s --docker-password=%s",
			"https://index.docker.io/v1/",
			os.Getenv("REGISTRY_USERNAME"),
			os.Getenv("REGISTRY_PASSWORD"),
		))
	}

	fmt.Printf("Installing Epinio on node %d\n", config.GinkgoConfig.ParallelNode)
	// Allow the installation to continue
	os.Setenv("EPINIO_DONT_WAIT_FOR_DEPLOYMENT", "1")
	out, err := installEpinio()
	Expect(err).ToNot(HaveOccurred(), out)

	os.Setenv("EPINIO_BINARY_PATH", path.Join(nodeTmpDir, "epinio"))
	// Patch Epinio deployment to inject the current binary
	out, err = RunProc("make patch-epinio-deployment", "..", false)
	Expect(err).ToNot(HaveOccurred(), out)

	// Now create the default org which we skipped because it would fail before
	// patching.
	// NOTE: Unfortunately this prevents us from testing if the `install` command
	// really creates a default workspace. Needs a better solution that allows
	// install to do it's thing without needing the patch script to run first.
	// Eventually is used to retry in case the rollout of the patched deployment
	// is not completely done yet.
	Eventually(func() error {
		_, err = Epinio("org create workspace", nodeTmpDir)
		return err
	}, "1m").ShouldNot(HaveOccurred())

	out, err = Epinio("target workspace", nodeTmpDir)
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = RunProc("kubectl get ingress -n epinio epinio -o=jsonpath='{.spec.rules[0].host}'", "..", false)
	Expect(err).ToNot(HaveOccurred(), out)

	serverURL = "http://" + out
})

var _ = AfterSuite(func() {
	fmt.Printf("Uninstall epinio on node %d\n", config.GinkgoConfig.ParallelNode)
	out, _ := uninstallEpinio()
	match, _ := regexp.MatchString(`Epinio uninstalled`, out)
	if !match {
		panic("Uninstalling epinio failed: " + out)
	}

	fmt.Printf("Deleting tmpdir on node %d\n", config.GinkgoConfig.ParallelNode)
	deleteTmpDir()
})

func ensureRegistryNetwork() (string, error) {
	out, err := RunProc(fmt.Sprintf("docker network create %s", networkName), "", false)
	if err != nil {
		alreadyExists, regexpErr := regexp.Match(`already exists`, []byte(out))
		if regexpErr != nil {
			return "", regexpErr
		}
		if alreadyExists {
			return "", nil
		}

		return "", err
	}

	return out, err
}

func ensureRegistryMirror() (string, error) {
	if os.Getenv(registryMirrorEnv) != "" {
		return "", nil
	}
	fmt.Println("Ensuring a registry mirror")

	out, err := RunProc(fmt.Sprintf("docker ps --filter name=%s -q", registryMirrorName), "", false)
	if err != nil {
		return out, err
	}
	if out == "" {
		fmt.Printf("Registry mirror %s is not running. I will try to create it.\n", registryMirrorName)
		command := fmt.Sprintf("docker run -d --network %s --name %s -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io -e REGISTRY_PROXY_USERNAME=%s -e REGISTRY_PROXY_PASSWORD=%s registry:2",
			networkName, registryMirrorName, os.Getenv("REGISTRY_USERNAME"), os.Getenv("REGISTRY_PASSWORD"))

		return RunProc(command, nodeTmpDir, false)
	}

	return out, err
}

func ensureCluster() {
	k3dConfigContents := fmt.Sprintf(`{"mirrors":{"docker.io":{"endpoint":["http://%s:5000"]}}}`, registryMirrorName)
	if os.Getenv(registryMirrorEnv) != "" {
		k3dConfigContents = os.Getenv(registryMirrorEnv)
		fmt.Printf("Using custom registry mirror config from '%s' for docker.io images\n", registryMirrorEnv)
	}

	tmpk3dConfig, err := helpers.CreateTmpFile(k3dConfigContents)
	if err != nil {
		panic(err.Error())
	}
	defer os.Remove(tmpk3dConfig)

	name := fmt.Sprintf("epinio-acceptance-%d", config.GinkgoConfig.ParallelNode)

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
			out, err := RunProc(
				fmt.Sprintf("k3d cluster create %s --registry-config %s --network %s",
					name, tmpk3dConfig, networkName),
				nodeTmpDir, false)
			if err != nil {
				panic(fmt.Sprintf("Creating k3d cluster failed: %s \n%s", err.Error(), out))
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
	name := fmt.Sprintf("epinio-acceptance-%s", nodeSuffix)

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

func buildEpinio() {
	output, err := RunProc("make", "..", false)
	if err != nil {
		panic(fmt.Sprintf("Couldn't build Epinio: %s\n %s\n"+err.Error(), output))
	}
}

func copyEpinio() {
	binary_path := "dist/epinio-" + runtime.GOOS + "-" + runtime.GOARCH
	output, err := RunProc("cp "+binary_path+" "+nodeTmpDir+"/epinio", "..", false)
	if err != nil {
		panic(fmt.Sprintf("Couldn't copy Epinio: %s\n %s\n"+err.Error(), output))
	}
}

func installEpinio() (string, error) {
	return Epinio("install --skip-default-org", "")
}

func uninstallEpinio() (string, error) {
	return Epinio("uninstall", "")
}

// Epinio invokes the `epinio` binary, running the specified command.
// It returns the command output and/or error.
// dir parameter defines the directory from which the command should be run.
// It defaults to the current dir if left empty.
func Epinio(command string, dir string) (string, error) {
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

	cmd := fmt.Sprintf(nodeTmpDir+"/epinio %s", command)

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
