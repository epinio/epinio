package machine

import (
	"fmt"
	"os"
	"path"

	. "github.com/onsi/gomega"
)

func (m *Machine) MakeApp(appName string, instances int, deployFromCurrentDir bool) string {
	currentDir, err := os.Getwd()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	appDir := path.Join(currentDir, m.root, "assets/sample-app")

	return m.MakeAppWithDir(appName, instances, deployFromCurrentDir, appDir)
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
		pushOutput, err = m.Epinio(fmt.Sprintf("apps push %s --instances %d", appName, instances), appDir)
	} else {
		// Note: appDir is handed as second argument to the epinio cli.
		// This means that the command gets the sources from that directory instead of CWD.
		pushOutput, err = m.Epinio(fmt.Sprintf("apps push %s %s --instances %d", appName, appDir, instances), "")
	}
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), pushOutput)

	// And check presence

	EventuallyWithOffset(1, func() string {
		out, err := m.Epinio("app list", "")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "5m").Should(MatchRegexp(fmt.Sprintf(`%s.*\|.*%d\/%d.*\|.*`, appName, instances, instances)))

	return pushOutput
}

func (m *Machine) DeleteApp(appName string) {
	out, err := m.Epinio("app delete "+appName, "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	// TODO: Fix `epinio delete` from returning before the app is deleted #131

	EventuallyWithOffset(1, func() string {
		out, err := m.Epinio("app list", "")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "1m").ShouldNot(MatchRegexp(`.*%s.*`, appName))
}

func (m *Machine) CleanupApp(appName string) {
	out, err := m.Epinio("app delete "+appName, "")
	// TODO: Fix `epinio delete` from returning before the app is deleted #131

	if err != nil {
		fmt.Printf("deleting app failed : %s\n%s", err.Error(), out)
	}
}
