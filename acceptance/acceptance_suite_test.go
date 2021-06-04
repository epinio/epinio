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
	"path/filepath"
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
	RegisterFailHandler(FailWithReport)
	RunSpecs(t, "Acceptance Suite")
}

var nodeSuffix, nodeTmpDir string

// serverURL is the URL of the epinio API server
var serverURL, websocketURL string
var registryMirrorName = "epinio-acceptance-registry-mirror"

const (
	networkName         = "epinio-acceptance"
	registryMirrorEnv   = "EPINIO_REGISTRY_CONFIG"
	registryUsernameEnv = "REGISTRY_USERNAME"
	registryPasswordEnv = "REGISTRY_PASSWORD"

	// skipCleanupPath is the path (relative to the test
	// directory) of a file which, when present causes the system
	// to not delete the test cluster after the tests are done.
	skipCleanupPath = "../tmp/skip_cleanup"

	// afterEachSleepPath is the path (relative to the test
	// directory) of a file which, when it, is readable, and
	// contains an integer number (*) causes the the system to
	// wait that many seconds after each test.
	//
	// (*) A number, i.e. just digits. __No trailing newline__
	afterEachSleepPath = "../tmp/after_each_sleep"

	kubeconfigPath = "../tmp/acceptance-kubeconfig"
	epinioYAML     = "../tmp/epinio.yaml"

	// k3dInstallArgsEnv contains the name of the environment
	// variable which, when present and not empty has its contents
	// added to the command creating the test cluster.  Does not
	// apply if an existing cluster is re-used.
	//
	// Example: -p '80:80@server[0]' -p '443:443@server[0]'
	k3dInstallArgsEnv = "EPINIO_K3D_INSTALL_ARGS"

	// skipEpinioPatch contains the name of the environment
	// variable which, when present and not empty causes system
	// startup to skip patching the epinio server pod. Best used
	// when the cluster of a previous run still exists
	// (s.a. skipCleanupPath).
	skipEpinioPatch = "EPINIO_SKIP_PATCH"

	// epinioUser and epinioPassword specify the API credentials
	// used during testing.
	epinioUser     = "test-user"
	epinioPassword = "secure-testing"
)

var _ = SynchronizedBeforeSuite(func() []byte {
	// Singleton setup. Run on node 1 before all

	fmt.Printf("I'm running on runner = %s\n", os.Getenv("HOSTNAME"))

	if os.Getenv(registryUsernameEnv) == "" || os.Getenv(registryPasswordEnv) == "" {
		fmt.Println("REGISTRY_USERNAME or REGISTRY_PASSWORD environment variables are empty. Pulling from dockerhub will be subject to rate limiting.")
	}

	if err := checkDependencies(); err != nil {
		panic("Missing dependencies: " + err.Error())
	}

	fmt.Printf("Compiling Epinio on node %d\n", config.GinkgoConfig.ParallelNode)
	buildEpinio()

	os.Setenv("EPINIO_BINARY_PATH", path.Join("dist", "epinio-linux-amd64"))
	os.Setenv("EPINIO_DONT_WAIT_FOR_DEPLOYMENT", "1")
	os.Setenv("EPINIO_CONFIG", epinioYAML)

	fmt.Println("Ensuring a docker network")
	out, err := ensureRegistryNetwork()
	Expect(err).ToNot(HaveOccurred(), out)
	out, err = ensureRegistryMirror()
	Expect(err).ToNot(HaveOccurred(), out)

	fmt.Println("Ensuring acceptance cluster")
	ensureCluster("epinio-acceptance")

	if os.Getenv(registryUsernameEnv) != "" && os.Getenv(registryPasswordEnv) != "" {
		fmt.Printf("Creating image pull secret for Dockerhub on node %d\n", config.GinkgoConfig.ParallelNode)
		_, _ = helpers.Kubectl(fmt.Sprintf("create secret docker-registry regcred --docker-server=%s --docker-username=%s --docker-password=%s",
			"https://index.docker.io/v1/",
			os.Getenv(registryUsernameEnv),
			os.Getenv(registryPasswordEnv),
		))
	}

	ensureEpinio()

	if os.Getenv(skipEpinioPatch) == "" {
		// Patch Epinio deployment to inject the current binary
		fmt.Println("Patching Epinio deployment with test binary")
		out, err = RunProc("make patch-epinio-deployment", "..", false)
		Expect(err).ToNot(HaveOccurred(), out)
	}

	// Now create the default org which we skipped because it would fail before
	// patching.
	// NOTE: Unfortunately this prevents us from testing if the `install` command
	// really creates a default workspace. Needs a better solution that allows
	// install to do it's thing without needing the patch script to run first.
	// Eventually is used to retry in case the rollout of the patched deployment
	// is not completely done yet.
	fmt.Println("Ensure default workspace exists")
	Eventually(func() error {
		out, err = RunProc("../dist/epinio-linux-amd64 org create workspace", "", false)
		if err != nil {
			if exists, err := regexp.Match(`Organization 'workspace' already exists`, []byte(out)); err == nil && exists {
				return nil
			}
		}
		return err
	}, "1m").ShouldNot(HaveOccurred(), out)

	fmt.Println("Setup cluster services")
	setupInClusterServices()
	out, err = helpers.Kubectl(`get pods -n minibroker --selector=app=minibroker-minibroker`)
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).To(MatchRegexp(`minibroker.*2/2.*Running`))

	fmt.Println("Setup google")
	setupGoogleServices()

	fmt.Println("SynchronizedBeforeSuite is done, checking Epinio info endpoint")
	expectGoodInstallation()

	return []byte(strconv.Itoa(int(time.Now().Unix())))
}, func(randomSuffix []byte) {
	var err error

	nodeSuffix = fmt.Sprintf("%d-%s",
		config.GinkgoConfig.ParallelNode, string(randomSuffix))
	nodeTmpDir, err = ioutil.TempDir("", "epinio-"+nodeSuffix)
	if err != nil {
		panic("Could not create temp dir: " + err.Error())
	}

	os.Setenv("KUBECONFIG", nodeTmpDir+"/kubeconfig")
	// Copy kubeconfig in the temp dir
	out, err := RunProc(fmt.Sprintf("cp %s %s/kubeconfig", kubeconfigPath, nodeTmpDir), "", false)
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = copyEpinio()
	Expect(err).ToNot(HaveOccurred(), out)

	fmt.Println("Waiting for kubernetes node to be ready")
	Eventually(func() error {
		out, err = waitUntilClusterNodeReady()
		return err
	}, "3m").ShouldNot(HaveOccurred(), out)

	os.Setenv("EPINIO_CONFIG", nodeTmpDir+"/epinio.yaml")

	// Get config from the installation (API credentials)
	out, err = RunProc(fmt.Sprintf("cp %s %s/epinio.yaml", epinioYAML, nodeTmpDir), "", false)
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = Epinio("target workspace", nodeTmpDir)
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = RunProc("kubectl get ingress -n epinio epinio -o=jsonpath='{.spec.rules[0].host}'", "..", false)
	Expect(err).ToNot(HaveOccurred(), out)

	serverURL = "http://" + out
	websocketURL = "ws://" + out
})

var _ = SynchronizedAfterSuite(func() {
	if !skipCleanup() {
		fmt.Printf("Deleting tmpdir on node %d\n", config.GinkgoConfig.ParallelNode)
		deleteTmpDir()
	}
}, func() { // Runs only on one node after all are done
	if skipCleanup() {
		fmt.Printf("Found '%s', skipping all cleanup", skipCleanupPath)
	} else {
		err := uninstallCluster()
		Expect(err).NotTo(HaveOccurred())

		// Delete left-overs no matter what
		defer func() { _, _ = cleanupTmp() }()
	}
})

var _ = AfterEach(func() {
	if _, err := os.Stat(afterEachSleepPath); err == nil {
		if data, err := ioutil.ReadFile(afterEachSleepPath); err == nil {
			if s, err := strconv.Atoi(string(data)); err == nil {
				t := time.Duration(s) * time.Second
				fmt.Printf("Found '%s', sleeping for '%s'", afterEachSleepPath, t)
				time.Sleep(t)
			}
		}
	}
})

// skipCleanup returns true if the file exists, false if some error occurred
// while checking
func skipCleanup() bool {
	_, err := os.Stat(skipCleanupPath)
	return err == nil
}

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
			networkName, registryMirrorName, os.Getenv(registryUsernameEnv), os.Getenv(registryPasswordEnv))

		return RunProc(command, nodeTmpDir, false)
	}

	return out, err
}

func ensureEpinio() {
	out, err := helpers.Kubectl(`get pods -n epinio --selector=app.kubernetes.io/name=epinio-server`)
	if err == nil {
		running, err := regexp.Match(`epinio-server.*Running`, []byte(out))
		if err != nil {
			return
		}
		if running {
			return
		}
	}
	fmt.Println("Installing Epinio")
	// Allow the installation to continue by not trying to create the default org
	// before we patch.
	out, err = RunProc(
		fmt.Sprintf("../dist/epinio-linux-amd64 install --skip-default-org --user %s --password %s", epinioUser, epinioPassword),
		"", false)
	Expect(err).ToNot(HaveOccurred(), out)
}

func uninstallCluster() error {
	_, err := RunProc("k3d cluster delete epinio-acceptance", "", false)
	return err
}

func ensureCluster(k3dClusterName string) {
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

	if _, err := exec.LookPath("k3d"); err != nil {
		panic("Couldn't find k3d in PATH: " + err.Error())
	}

	out, err := RunProc("k3d cluster get "+k3dClusterName, "", false)
	if err != nil {
		notExists, regexpErr := regexp.Match(`No nodes found for given cluster`, []byte(out))
		if regexpErr != nil {
			panic(regexpErr)
		}
		if notExists {
			fmt.Printf("k3d cluster %s doesn't exist. I will try to create it.\n", k3dClusterName)
			out, err := RunProc(
				fmt.Sprintf("k3d cluster create %s --registry-config %s --network %s %s"+
					" --k3s-server-arg --disable --k3s-server-arg traefik",
					k3dClusterName, tmpk3dConfig, networkName, os.Getenv(k3dInstallArgsEnv)),
				"", false)
			if err != nil {
				panic(fmt.Sprintf("Creating k3d cluster failed: %s \n%s", err.Error(), out))
			}
		} else {
			panic("Looking up k3d cluster failed: " + err.Error())
		}
	}

	// Ensure we are talking to the correct cluster
	kubeconfig, err := RunProc("k3d kubeconfig get epinio-acceptance", "", false)
	if err != nil {
		panic("Getting kubeconfig failed: " + err.Error())
	}
	if err := os.MkdirAll("../tmp", 0755); err != nil {
		panic("cannot create tmp dir")
	}
	err = ioutil.WriteFile(kubeconfigPath, []byte(kubeconfig), 0644)
	if err != nil {
		panic("Writing kubeconfig failed: " + err.Error())
	}
	p, err := filepath.Abs(kubeconfigPath)
	Expect(err).ToNot(HaveOccurred())
	os.Setenv("KUBECONFIG", p)
}

func waitUntilClusterNodeReady() (string, error) {
	nodeName, err := RunProc("kubectl get nodes -o name", nodeTmpDir, true)
	if err != nil {
		return nodeName, err
	}

	return RunProc("kubectl wait --for=condition=Ready "+nodeName, nodeTmpDir, true)
}

func deleteTmpDir() {
	err := os.RemoveAll(nodeTmpDir)
	if err != nil {
		panic(fmt.Sprintf("Failed deleting temp dir %s: %s\n",
			nodeTmpDir, err.Error()))
	}
}

func GetProc(command string, dir string) (*kexec.KCommand, error) {
	var commandDir string
	var err error

	if dir == "" {
		commandDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	} else {
		commandDir = dir
	}

	p := kexec.CommandString(command)
	p.Dir = commandDir

	return p, nil
}

func RunProc(cmd, dir string, toStdout bool) (string, error) {
	p, err := GetProc(cmd, dir)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	if toStdout {
		p.Stdout = io.MultiWriter(os.Stdout, &b)
		p.Stderr = io.MultiWriter(os.Stderr, &b)
	} else {
		p.Stdout = &b
		p.Stderr = &b
	}

	if err := p.Run(); err != nil {
		return b.String(), err
	}

	err = p.Wait()
	return b.String(), err
}

func buildEpinio() {
	output, err := RunProc("make", "..", false)
	if err != nil {
		panic(fmt.Sprintf("Couldn't build Epinio: %s\n %s\n"+err.Error(), output))
	}
}

func copyEpinio() (string, error) {
	binaryPath := "dist/epinio-" + runtime.GOOS + "-" + runtime.GOARCH
	return RunProc("cp "+binaryPath+" "+nodeTmpDir+"/epinio", "..", false)
}

// Remove all tmp directories from /tmp/epinio-* . Test should try to cleanup
// after themselves but that sometimes doesn't happen, either because we forgot
// the cleanup code or because the test failed before that happened.
// NOTE: This code will create problems if more than one acceptance_suite_test.go
// is run in parallel (e.g. two PRs on one worker). However we keep it as an
// extra measure.
func cleanupTmp() (string, error) {
	return RunProc("rm -rf /tmp/epinio-*", "", true)
}

// Epinio invokes the `epinio` binary, running the specified command.
// It returns the command output and/or error.
// dir parameter defines the directory from which the command should be run.
// It defaults to the current dir if left empty.
func Epinio(command string, dir string) (string, error) {
	cmd := fmt.Sprintf(nodeTmpDir+"/epinio %s", command)
	return RunProc(cmd, dir, false)
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

func expectGoodInstallation() {
	info, err := RunProc("../dist/epinio-linux-amd64 info", "", false)
	Expect(err).ToNot(HaveOccurred())
	Expect(info).To(MatchRegexp("Platform: k3s"))
	Expect(info).To(MatchRegexp("Kubernetes Version: v1.20"))
	Expect(info).To(MatchRegexp("Gitea Version: unavailable"))
}

func setupGoogleServices() {
	serviceAccountJSON, err := helpers.CreateTmpFile(`
				{
					"type": "service_account",
					"project_id": "myproject",
					"private_key_id": "somekeyid",
					"private_key": "someprivatekey",
					"client_email": "client@example.com",
					"client_id": "clientid",
					"auth_uri": "https://accounts.google.com/o/oauth2/auth",
					"token_uri": "https://oauth2.googleapis.com/token",
					"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
					"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/client%40example.com"
				}
			`)
	Expect(err).ToNot(HaveOccurred(), serviceAccountJSON)

	defer os.Remove(serviceAccountJSON)

	out, err := RunProc("../dist/epinio-linux-amd64 enable services-google --service-account-json "+serviceAccountJSON, "", false)
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = helpers.Kubectl(`get pods -n google-service-broker --selector=app.kubernetes.io/name=gcp-service-broker`)
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).To(MatchRegexp(`google-service-broker-gcp-service-broker.*2/2.*Running`))
}
