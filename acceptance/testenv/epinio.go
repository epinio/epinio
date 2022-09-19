package testenv

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/epinio/epinio/acceptance/helpers/machine"
	"github.com/epinio/epinio/acceptance/helpers/proc"

	ginkgo "github.com/onsi/ginkgo/v2"
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

	// Namespace is the namespace used for the epinio server and staging setup
	Namespace = "epinio"
)

type EpinioEnv struct {
	machine.Machine
	nodeTmpDir       string
	EpinioUser       string
	EpinioPassword   string
	EpinioAdminToken string
	EpinioUserToken  string
}

func New(nodeDir string, rootDir, username, password, adminToken, userToken string) EpinioEnv {
	ginkgo.By(fmt.Sprintf("TestEnv (n=%s, r=%s, u=%s, p=%s, at=%s, ut=%s)",
		nodeDir, rootDir, username, password, adminToken, userToken))
	return EpinioEnv{
		nodeTmpDir:       nodeDir,
		EpinioUser:       username,
		EpinioPassword:   password,
		EpinioAdminToken: adminToken,
		EpinioUserToken:  userToken,
		Machine:          machine.New(nodeDir, username, password, adminToken, userToken, root, EpinioBinaryPath()),
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
	if _, ok := os.LookupEnv("EPINIO_COVERAGE"); ok {
		p, _ := filepath.Abs(filepath.Join(Root(), "epinio.test"))
		ginkgo.By("CLI Binary (Env): " + p)
		return p
	}
	p, _ := filepath.Abs(filepath.Join(Root(), "dist", BinaryName()))
	ginkgo.By("CLI Binary (Sys): " + p)
	return p
}

// EpinioYAML returns the absolute path to the epinio settings YAML
func EpinioYAML() string {
	path := os.Getenv("EPINIO_SETTINGS")
	if path == "" {
		fmt.Printf("YAML Default!\n")
		path = os.ExpandEnv("${HOME}/.config/epinio/settings.yaml")
	}

	fmt.Printf("YAML: %s\n", path)
	return path
}

// BuildEpinio builds the epinio binaries for the server and if platforms are different also for the CLI
func BuildEpinio() {
	targets := []string{"build-linux-amd64"}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		// we need a different binary to run locally
		targets = append(targets, fmt.Sprintf("build-%s-%s", runtime.GOOS, runtime.GOARCH))
	}

	ginkgo.By(fmt.Sprintf("Compiling epinio from local checkout: %v", targets))

	output, err := proc.Run(Root(), false, "make", targets...)
	if err != nil {
		panic(fmt.Sprintf("Couldn't build Epinio: %s\n %s\n"+err.Error(), output))
	}
	ginkgo.By("Done compiling")
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
	routeRegexp := regexp.MustCompile(`Routes: .*\n.*(https:\/\/.*\.omg\.howdoi\.website)`)
	return routeRegexp.FindStringSubmatch(out)[1]
}
