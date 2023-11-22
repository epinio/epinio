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

package machine

import (
	"fmt"
	"os"
	"path"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (m *Machine) MakeApp(appName string, instances int, deployFromCurrentDir bool) string {
	currentDir, err := os.Getwd()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	appDir := path.Join(currentDir, m.root, "assets/sample-app")

	return m.MakeAppWithDir(appName, instances, deployFromCurrentDir, appDir)
}

func (m *Machine) MakeContainerImageApp(appName string, instances int, containerImageURL string) string {
	By("making a container app: " + appName)

	pushOutput, err := m.Epinio("", "apps", "push",
		"--name", appName,
		"--container-image-url", containerImageURL,
		"--instances", strconv.Itoa(instances))
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), pushOutput)

	EventuallyWithOffset(1, func() string {
		out, err := m.Epinio("", "app", "list")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "5m").Should(MatchRegexp(fmt.Sprintf(`%s.*\|.*%d\/%d.*\|.*`, appName, instances, instances)))

	return pushOutput
}

func (m *Machine) MakeRoutedContainerImageApp(appName string, instances int, containerImageURL, route string, more ...string) string {
	By("making a routed container app: " + appName)

	pushOutput, err := m.Epinio("", "apps", append([]string{
		"push",
		"--name", appName,
		"--route", route,
		"--container-image-url", containerImageURL,
		"--instances", strconv.Itoa(instances),
	}, more...)...)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), pushOutput)

	EventuallyWithOffset(1, func() string {
		out, err := m.Epinio("", "app", "list")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "5m").Should(MatchRegexp(fmt.Sprintf(`%s.*\|.*%d\/%d.*\|.*`, appName, instances, instances)))

	return pushOutput
}

func (m *Machine) MakeGolangApp(appName string, instances int, deployFromCurrentDir bool) string {
	currentDir, err := os.Getwd()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	appDir := path.Join(currentDir, m.root, "assets/golang-sample-app")

	return m.MakeAppWithDir(appName, instances, deployFromCurrentDir, appDir)
}

func (m *Machine) MakeAppWithDir(appName string, instances int, deployFromCurrentDir bool, appDir string, more ...string) string {
	By("creating app " + appName)

	var pushOutput string
	var err error

	if deployFromCurrentDir {
		// Note: appDir is handed to the working dir argument of Epinio().
		// This means that the command runs with it as the CWD.
		pushOutput, err = m.EpinioPush(appDir,
			appName,
			append([]string{
				"--name", appName,
				"--instances", strconv.Itoa(instances),
			}, more...)...)
	} else {
		// Note: appDir is handed as second argument to the epinio cli.
		// This means that the command gets the sources from that directory instead of CWD.
		pushOutput, err = m.EpinioPush("",
			appName,
			"--name", appName,
			"--path", appDir,
			"--instances", strconv.Itoa(instances))
	}

	Expect(err).ToNot(HaveOccurred(), pushOutput)

	// Check presence if necessary, i.e. expected to be present
	if instances > 0 {
		Eventually(func() string {
			out, err := m.Epinio("", "app", "list")
			Expect(err).ToNot(HaveOccurred(), out)
			return out
		}, "5m").Should(MatchRegexp(fmt.Sprintf(`%s.*\|.*%d\/%d.*\|.*`, appName, instances, instances)))
	}

	return pushOutput
}

func (m *Machine) MakeAppWithDirSimple(appName string, deployFromCurrentDir bool, appDir string) string {
	var pushOutput string
	var err error

	if deployFromCurrentDir {
		// Note: appDir is handed to the working dir argument of Epinio().
		// This means that the command runs with it as the CWD.
		pushOutput, err = m.EpinioPush(appDir, appName, "--name", appName)
	} else {
		// Note: appDir is handed as second argument to the epinio cli.
		// This means that the command gets the sources from that directory instead of CWD.
		pushOutput, err = m.EpinioPush("", appName, "--name", appName, "--path", appDir)
	}

	ExpectWithOffset(1, err).ToNot(HaveOccurred(), pushOutput)

	// And check presence
	EventuallyWithOffset(1, func() string {
		out, err := m.Epinio("", "app", "list")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "5m").Should(MatchRegexp(fmt.Sprintf(`%s.*\|.*\/.*\|.*`, appName)))

	return pushOutput
}

func (m *Machine) DeleteApp(appName string) {
	By("deleting app " + appName)

	out, err := m.Epinio("", "app", "delete", appName)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	EventuallyWithOffset(1, func() string {
		out, err := m.Epinio("", "app", "list")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "1m").ShouldNot(MatchRegexp(`.*%s.*`, appName))
}

func (m *Machine) CleanupApp(appName string) {
	out, err := m.Epinio("", "app", "delete", appName)
	if err != nil {
		fmt.Printf("deleting app failed : %s\n%s", err.Error(), out)
	}
}
