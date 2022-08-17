package machine

import (
	"fmt"
	"os"
	"path"
	"strconv"

	. "github.com/onsi/gomega"
)

func (m *Machine) MakeApp(appName string, instances int, deployFromCurrentDir bool) string {
	currentDir, err := os.Getwd()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	appDir := path.Join(currentDir, m.root, "assets/sample-app")

	return m.MakeAppWithDir(appName, instances, deployFromCurrentDir, appDir)
}

func (m *Machine) MakeContainerImageApp(appName string, instances int, containerImageURL string) string {
	pushOutput := m.Epinio("", "apps", "push",
		"--name", appName,
		"--container-image-url", containerImageURL,
		"--instances", strconv.Itoa(instances))

	EventuallyWithOffset(1,
		m.Epinio("", "app", "list"),
		"5m",
	).Should(MatchRegexp(fmt.Sprintf(`%s.*\|.*%d\/%d.*\|.*`, appName, instances, instances)))

	return pushOutput
}

func (m *Machine) MakeRoutedContainerImageApp(appName string, instances int, containerImageURL, route string, more ...string) string {
	pushOutput := m.Epinio("", "apps", append([]string{
		"push",
		"--name", appName,
		"--route", route,
		"--container-image-url", containerImageURL,
		"--instances", strconv.Itoa(instances),
	}, more...)...)

	EventuallyWithOffset(1,
		m.Epinio("", "app", "list"),
		"5m",
	).Should(MatchRegexp(fmt.Sprintf(`%s.*\|.*%d\/%d.*\|.*`, appName, instances, instances)))

	return pushOutput
}

func (m *Machine) MakeGolangApp(appName string, instances int, deployFromCurrentDir bool) string {
	currentDir, err := os.Getwd()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	appDir := path.Join(currentDir, m.root, "assets/golang-sample-app")

	return m.MakeAppWithDir(appName, instances, deployFromCurrentDir, appDir)
}

func (m *Machine) MakeAppWithDir(appName string, instances int, deployFromCurrentDir bool, appDir string) string {
	var pushOutput string
	var err error

	if deployFromCurrentDir {
		// Note: appDir is handed to the working dir argument of Epinio().
		// This means that the command runs with it as the CWD.
		pushOutput, err = m.EpinioPush(appDir,
			appName,
			"--name", appName,
			"--instances", strconv.Itoa(instances))
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
		Eventually(m.Epinio("", "app", "list"), "5m").
			Should(MatchRegexp(fmt.Sprintf(`%s.*\|.*%d\/%d.*\|.*`, appName, instances, instances)))
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
	EventuallyWithOffset(1,
		m.Epinio("", "app", "list"),
		"5m",
	).Should(MatchRegexp(fmt.Sprintf(`%s.*\|.*\/.*\|.*`, appName)))

	return pushOutput
}

func (m *Machine) DeleteApp(appName string) {
	_ = m.Epinio("", "app", "delete", appName)

	EventuallyWithOffset(1,
		m.Epinio("", "app", "list"),
		"1m",
	).ShouldNot(MatchRegexp(`.*%s.*`, appName))
}

func (m *Machine) CleanupApp(appName string) {
	_ = m.Epinio("", "app", "delete", appName)
}
