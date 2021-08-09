package testenv

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"

	"github.com/epinio/epinio/acceptance/helpers/machine"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/helpers"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"
)

const (
	networkName         = "epinio-acceptance"
	registryMirrorEnv   = "EPINIO_REGISTRY_CONFIG"
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
		Machine:        machine.New(nodeDir, epinioUser, epinioPassword, root),
	}
}

func (e *EpinioEnv) CopyEpinio() (string, error) {
	binaryPath := path.Join("dist", "epinio-"+runtime.GOOS+"-"+runtime.GOARCH)
	return proc.Run("cp "+binaryPath+" "+e.nodeTmpDir+"/epinio", Root(), false)
}

func EpinioYAML() string {
	return Root() + "/tmp/epinio.yaml"
}

func EnsureEpinio(epinioBinary string) error {
	out, err := helpers.Kubectl(`get pods -n epinio --selector=app.kubernetes.io/name=epinio-server`)
	if err == nil {
		running, err := regexp.Match(`epinio-server.*Running`, []byte(out))
		if err != nil {
			return err
		}
		if running {
			return nil
		}
	}
	fmt.Println("Installing Epinio")

	// Installing linkerd and ingress separate from the main
	// pieces.  Ensures that the main install command invokes and
	// passes the presence checks for linkerd and traefik.
	out, err = proc.Run(fmt.Sprintf("%s%s install-ingress", Root(), epinioBinary),
		"", false)
	if err != nil {
		return errors.Wrap(err, out)
	}

	domainSetting := ""
	if domain := os.Getenv("EPINIO_SYSTEM_DOMAIN"); domain != "" {
		domainSetting = fmt.Sprintf(" --system-domain %s", domain)
	}

	// Allow the installation to continue by not trying to create the default org
	// before we patch.
	out, err = proc.Run(
		fmt.Sprintf("%s%s install --skip-default-org --user %s --password %s %s", Root(), epinioBinary, epinioUser, epinioPassword, domainSetting),
		"", false)
	if err != nil {
		return errors.Wrap(err, out)
	}
	return nil
}

func BuildEpinio() {
	output, err := proc.Run("make", Root(), false)
	if err != nil {
		panic(fmt.Sprintf("Couldn't build Epinio: %s\n %s\n"+err.Error(), output))
	}
}

func ExpectGoodInstallation(epinioBinary string) {
	info, err := proc.Run(fmt.Sprintf("%s%s info", Root(), epinioBinary), "", false)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(info).To(gomega.Or(
		gomega.MatchRegexp("Kubernetes Version: v1.21"),
		gomega.MatchRegexp("Kubernetes Version: v1.20"),
		gomega.MatchRegexp("Kubernetes Version: v1.19")))
	gomega.Expect(info).To(gomega.MatchRegexp("Gitea Version: unavailable"))
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

func EnsureDefaultWorkspace(epinioBinary string) {
	gomega.Eventually(func() string {
		out, err := proc.Run(fmt.Sprintf("%s%s org create workspace", Root(), epinioBinary), "", false)
		if err != nil {
			if exists, err := regexp.Match(`Organization 'workspace' already exists`, []byte(out)); err == nil && exists {
				return ""
			}
			return errors.Wrap(err, out).Error()
		}
		return ""
	}, "1m").Should(gomega.BeEmpty())
}
