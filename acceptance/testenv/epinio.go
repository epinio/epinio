// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/machine"
	"github.com/epinio/epinio/acceptance/helpers/proc"
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
	p, _ := filepath.Abs(filepath.Join(Root(), "dist", BinaryName()))
	return p
}

// EpinioYAML returns the absolute path to the epinio settings YAML
func EpinioYAML() string {
	if os.Getenv("EPINIO_SETTINGS") == "" {
		return os.ExpandEnv("${HOME}/.config/epinio/settings.yaml")
	}

	return os.Getenv("EPINIO_SETTINGS")
}

// BuildEpinio builds the epinio binaries for the server and if platforms are different also for the CLI
func BuildEpinio() {
	targets := []string{"build-linux-amd64"}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		// we need a different binary to run locally
		targets = append(targets, fmt.Sprintf("build-%s-%s", runtime.GOOS, runtime.GOARCH))
	}

	output, err := proc.Run(Root(), false, "make", targets...)
	if err != nil {
		panic(fmt.Sprintf("Couldn't build Epinio: %s\n %s\n"+err.Error(), output))
	}
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
	// Match route URL with either sslip.io or nip.io (environment-specific)
	routeRegexp := regexp.MustCompile(`Routes: .*\n.*(https:\/\/[^\s]+(?:sslip\.io|nip\.io))`)
	matches := routeRegexp.FindStringSubmatch(out)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// AppRouteWithPort ensures the route URL has the correct port for the environment.
// In k3d, the loadbalancer maps host port 8443 to container 443, so app routes
// must use :8443 when curling from the host. Routes without a port default to 443
// which causes "connection refused".
func AppRouteWithPort(route string) string {
	if route == "" {
		return route
	}
	port := os.Getenv("EPINIO_PORT")
	if port == "" {
		port = "8443" // k3d default
	}
	// If route already has a port, return as-is (avoid double-adding)
	if regexp.MustCompile(`:\d+/?($|\?)`).MatchString(route) {
		return route
	}
	// Insert port after hostname (after "://" and up to path or end)
	schemeEnd := strings.Index(route, "://")
	if schemeEnd < 0 {
		return route + ":" + port
	}
	hostStart := schemeEnd + 3 // after "://"
	pathIdx := strings.Index(route[hostStart:], "/")
	if pathIdx < 0 {
		return route + ":" + port
	}
	insertAt := hostStart + pathIdx
	return route[:insertAt] + ":" + port + route[insertAt:]
}
