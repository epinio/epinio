// Package machine provides a number of utility functions encapsulating often-used sequences.
// I.e. create/delete applications/services, bind/unbind services, etc.
// This is done in the hope of enhancing the readability of various before/after blocks.
package machine

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/cli/config"
	"github.com/gorilla/websocket"
)

type Machine struct {
	nodeTmpDir       string
	user             string
	password         string
	root             string
	epinioBinaryPath string
}

func New(dir string, user string, password string, root string, epinioBinaryPath string) Machine {
	return Machine{dir, user, password, root, epinioBinaryPath}
}

// Epinio invokes the `epinio` binary, running the specified command.
// It returns the command output and/or error.
// dir parameter defines the directory from which the command should be run.
// It defaults to the current dir if left empty.
func (m *Machine) Epinio(dir, command string, arg ...string) (string, error) {
	return proc.Run(dir, false, m.epinioBinaryPath, append([]string{command}, arg...)...)
}

func (m *Machine) SetupAndTargetOrg(org string) {
	By("creating a namespace")

	out, err := m.Epinio("", "namespace", "create", org)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio("", "namespace", "show", org)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, out).To(MatchRegexp("Name.*|.*" + org))

	m.TargetOrg(org)
}

func (m *Machine) TargetOrg(org string) {
	By("targeting a namespace")

	out, err := m.Epinio(m.nodeTmpDir, "target", org)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio(m.nodeTmpDir, "target")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp("Currently targeted namespace: " + org))
}

func (m *Machine) DeleteNamespace(namespace string) {
	By("deleting a namespace")

	out, err := m.Epinio("", "namespace", "delete", "-f", namespace)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio("", "namespace", "show", namespace)
	ExpectWithOffset(1, err).To(HaveOccurred())
	ExpectWithOffset(1, out).To(MatchRegexp(".*Not Found: Targeted namespace.*does not exist.*"))
}

func (m *Machine) VerifyOrgNotExist(org string) {
	out, err := m.Epinio("", "namespace", "list")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).ToNot(MatchRegexp(org))
}

func (m *Machine) MakeWebSocketConnection(url string) *websocket.Conn {
	headers := http.Header{
		"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", m.user, m.password)))},
	}

	// disable tls cert verification for web socket connections - See also `Curl` above
	websocket.DefaultDialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, // nolint:gosec // tests using self signed certs
	}
	ws, response, err := websocket.DefaultDialer.Dial(url, headers)
	Expect(err).NotTo(HaveOccurred())
	Expect(response.StatusCode).To(Equal(http.StatusSwitchingProtocols))
	return ws
}

func (m *Machine) GetPodNames(appName, orgName string) []string {
	jsonPath := `{range .items[*]}{.metadata.name}{"\n"}{end}`
	out, err := helpers.Kubectl("get", "pods",
		"--namespace", orgName,
		"--selector", fmt.Sprintf("app.kubernetes.io/component=application,app.kubernetes.io/name=%s, app.kubernetes.io/part-of=%s", appName, orgName),
		"-o", "jsonpath="+jsonPath)
	Expect(err).NotTo(HaveOccurred())

	return strings.Split(out, "\n")
}

func (m *Machine) GetConfigFrom(location string) (*config.Config, error) {
	return config.LoadFrom(location)
}

func (m *Machine) GetConfig() (*config.Config, error) {
	return config.Load()
}
