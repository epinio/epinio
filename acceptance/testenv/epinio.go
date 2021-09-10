package testenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/epinio/epinio/acceptance/helpers/machine"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/helpers"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"
)

const (
	registryUsernameEnv = "REGISTRY_USERNAME"
	registryPasswordEnv = "REGISTRY_PASSWORD"

	// afterEachSleepPath is the path (relative to the test
	// directory) of a file which, when it, is readable, and
	// contains an integer number (*) causes the the system to
	// wait that many seconds after each test.
	//
	// (*) A number, i.e. just digits. __No trailing newline__
	afterEachSleepPath = "/tmp/after_each_sleep"

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

type EpinioEnv struct {
	machine.Machine
	nodeTmpDir     string
	EpinioUser     string
	EpinioPassword string
}

func New(nodeDir string, rootDir string) EpinioEnv {
	return EpinioEnv{
		nodeTmpDir:     nodeDir,
		EpinioUser:     epinioUser,
		EpinioPassword: epinioPassword,
		Machine:        machine.New(nodeDir, epinioUser, epinioPassword, root, EpinioBinaryPath()),
	}
}

// BinaryName returns the name of the epinio binary for the current platform
func BinaryName() string {
	return fmt.Sprintf("epinio-%s-%s", runtime.GOOS, runtime.GOARCH)
}

// ServerBinaryName returns the name of the epinio binary for the server
// platform. Currently only linux servers are supported.
func ServerBinaryName() string {
	return "epinio-linux-amd64"
}

// EpinioBinaryPath returns the absolute path to the dist/epinio binary
func EpinioBinaryPath() string {
	p, _ := filepath.Abs(filepath.Join(Root(), "dist", BinaryName()))
	return p
}

// EpinioYAML returns the absolute path to the epinio config YAML
func EpinioYAML() string {
	return os.ExpandEnv("${HOME}/.config/epinio/config.yaml")
}

func CheckEpinio() error {
	out, err := helpers.Kubectl("get", "pods",
		"--namespace", "epinio",
		"--selector", "app.kubernetes.io/name=epinio-server")
	if err == nil {
		running, err := regexp.Match(`epinio-server.*Running`, []byte(out))
		if err != nil {
			return err
		}
		if running {
			return nil
		}
	}

	return errors.New("epinio installation not found")
}

// BuildEpinio builds the epinio binaries for the server and if platforms are different also for the CLI
func BuildEpinio() {
	targets := []string{"embed_files", "build-linux-amd64"}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		// we need a different binary to run locally
		targets = append(targets, fmt.Sprintf("build-%s-%s", runtime.GOOS, runtime.GOARCH))
	}

	output, err := proc.Run(Root(), false, "make", targets...)
	if err != nil {
		panic(fmt.Sprintf("Couldn't build Epinio: %s\n %s\n"+err.Error(), output))
	}
}

func ExpectGoodInstallation(epinioBinary string) {
	info, err := proc.Run("", false, epinioBinary, "info")
	gomega.Expect(err).ToNot(gomega.HaveOccurred(), info)
	gomega.Expect(info).To(gomega.Or(
		gomega.MatchRegexp("Kubernetes Version: v1.21"),
		gomega.MatchRegexp("Kubernetes Version: v1.20"),
		gomega.MatchRegexp("Kubernetes Version: v1.19")))
}

func CheckDependencies() error {
	ok := true

	dependencies := []struct {
		CommandName string
	}{
		{CommandName: "wget"},
		{CommandName: "tar"},
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

const DefaultWorkspace = "workspace"

func EnsureDefaultWorkspace(epinioBinary string) {
	gomega.Eventually(func() string {
		out, err := proc.Run("", false, epinioBinary, "namespace", "create", DefaultWorkspace)
		if err != nil {
			if exists, err := regexp.Match(`Namespace 'workspace' already exists`, []byte(out)); err == nil && exists {
				return ""
			}
			return errors.Wrap(err, out).Error()
		}
		return ""
	}, "1m").Should(gomega.BeEmpty())
}

func AppRouteFromOutput(out string) string {
	routeRegexp := regexp.MustCompile(`Route: (https:\/\/.*\.omg\.howdoi\.website)`)
	return routeRegexp.FindStringSubmatch(out)[1]
}
