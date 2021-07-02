// Package machine provides a number of utility functions encapsulating often-used sequences.
// I.e. create/delete applications/services, bind/unbind services, etc.
// This is done in the hope of enhancing the readability of various before/after blocks.
package machine

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/cli/config"
	"github.com/gorilla/websocket"
)

type Machine struct {
	nodeTmpDir string
	user       string
	password   string
	root       string
}

func New(dir string, user string, password string, root string) Machine {
	return Machine{dir, user, password, root}
}

// Epinio invokes the `epinio` binary, running the specified command.
// It returns the command output and/or error.
// dir parameter defines the directory from which the command should be run.
// It defaults to the current dir if left empty.
func (m *Machine) Epinio(command string, dir string) (string, error) {
	cmd := fmt.Sprintf(m.nodeTmpDir+"/epinio %s", command)
	return proc.Run(cmd, dir, false)
}

func (m *Machine) SetupAndTargetOrg(org string) {
	By("creating an org")

	out, err := m.Epinio("org create "+org, "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	orgs, err := m.Epinio("org list", "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, orgs).To(MatchRegexp(org))

	By("targeting an org")

	out, err = m.Epinio(fmt.Sprintf("target %s", org), m.nodeTmpDir)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio("target", m.nodeTmpDir)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp("Currently targeted organization: " + org))
}

func (m *Machine) VerifyOrgNotExist(org string) {
	out, err := m.Epinio("org list", "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).ToNot(MatchRegexp(org))
}

func (m *Machine) MakeWebSocketConnection(url string) *websocket.Conn {
	headers := http.Header{
		"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", m.user, m.password)))},
	}

	// disable tls cert verification for web socket connections - See also `Curl` above
	websocket.DefaultDialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, // self signed certs
	}
	ws, response, err := websocket.DefaultDialer.Dial(url, headers)
	Expect(err).NotTo(HaveOccurred())
	Expect(response.StatusCode).To(Equal(http.StatusSwitchingProtocols))
	return ws
}

func (m *Machine) GetPodNames(appName, orgName string) []string {
	jsonPath := `'{range .items[*]}{.metadata.name}{"\n"}{end}'`
	out, err := helpers.Kubectl(fmt.Sprintf("get pods -n %s --selector 'app.kubernetes.io/component=application,app.kubernetes.io/name=%s, app.kubernetes.io/part-of=%s' -o=jsonpath=%s", orgName, appName, orgName, jsonPath))
	Expect(err).NotTo(HaveOccurred())

	return strings.Split(out, "\n")
}

func (m *Machine) GetConfigFrom(location string) (*config.Config, error) {
	os.Setenv("EPINIO_CONFIG", location)
	defer func() {
		os.Setenv("EPINIO_CONFIG", m.nodeTmpDir+"/epinio.yaml")
	}()
	return config.Load()
}

func (m *Machine) GetConfig() (*config.Config, error) {
	return config.Load()
}
