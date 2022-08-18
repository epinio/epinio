// Package machine provides a number of utility functions encapsulating often-used sequences.
// I.e. create/delete applications/configurations, bind/unbind configurations, etc.
// This is done in the hope of enhancing the readability of various before/after blocks.
package machine

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	urlpkg "net/url"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/gorilla/websocket"
)

type Machine struct {
	nodeTmpDir       string
	token            string
	root             string
	epinioBinaryPath string
}

func New(dir string, token string, root string, epinioBinaryPath string) Machine {
	return Machine{dir, token, root, epinioBinaryPath}
}

func (m *Machine) ShowStagingLogs(app string) {
	_, _ = m.Epinio("", app, "app", "logs", "--staging", app)
}

// Epinio invokes the `epinio` binary, running the specified command.
// It returns the command output and/or error.
// dir parameter defines the directory from which the command should be run.
// It defaults to the current dir if left empty.
func (m *Machine) Epinio(dir, command string, arg ...string) (string, error) {
	return proc.Run(dir, false, m.epinioBinaryPath, append([]string{command}, arg...)...)
}

const stagingError = "Failed to stage"

// EpinioPush shows the staging log if the error indicates that staging
// failed
func (m *Machine) EpinioPush(dir string, name string, arg ...string) (string, error) {
	out, err := proc.Run(dir, false, m.epinioBinaryPath, append([]string{"apps", "push"}, arg...)...)
	if err != nil && strings.Contains(out, stagingError) {
		m.ShowStagingLogs(name)
	}

	return out, err
}

func (m *Machine) SetupNamespace(namespace string) {
	By(fmt.Sprintf("creating a namespace: %s", namespace))

	out, err := m.Epinio("", "namespace", "create", namespace)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio("", "namespace", "show", namespace)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, out).To(MatchRegexp("Name.*|.*" + namespace))
}

func (m *Machine) TargetNamespace(namespace string) {
	By(fmt.Sprintf("targeting a namespace: %s", namespace))

	out, err := m.Epinio(m.nodeTmpDir, "target", namespace)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio(m.nodeTmpDir, "target")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp("Currently targeted namespace: " + namespace))
}

func (m *Machine) SetupAndTargetNamespace(namespace string) {
	m.SetupNamespace(namespace)
	m.TargetNamespace(namespace)
}

func (m *Machine) DeleteNamespace(namespace string) {
	By(fmt.Sprintf("deleting a namespace: %s", namespace))

	out, err := m.Epinio("", "namespace", "delete", "-f", namespace)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio("", "namespace", "show", namespace)
	ExpectWithOffset(1, err).To(HaveOccurred())
	ExpectWithOffset(1, out).To(MatchRegexp(".*Not Found: namespace '" + namespace + "' does not exist.*"))
}

func (m *Machine) VerifyNamespaceNotExist(namespace string) {
	out, err := m.Epinio("", "namespace", "list")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).ToNot(MatchRegexp(namespace))
}

func (m *Machine) MakeWebSocketConnection(authToken string, url string, subprotocols ...string) (*websocket.Conn, error) {
	u, err := urlpkg.Parse(url)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing url: %s", url)
	}

	values := u.Query()
	values.Add("authtoken", authToken)
	u.RawQuery = values.Encode()

	// disable tls cert verification for web socket connections - See also `Curl` above
	websocket.DefaultDialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, // nolint:gosec // tests using self signed certs
	}

	dialer := websocket.DefaultDialer
	dialer.Subprotocols = subprotocols
	ws, response, err := dialer.Dial(u.String(), http.Header{})

	var b []byte
	if response != nil {
		b, _ = ioutil.ReadAll(response.Body)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "Dialing endpoint. Response: %s", string(b))
	}

	Expect(response.StatusCode).To(Equal(http.StatusSwitchingProtocols))

	return ws, nil
}

func (m *Machine) GetPodNames(appName, namespaceName string) []string {
	jsonPath := `{range .items[*]}{.metadata.name}{"\n"}{end}`
	out, err := proc.Kubectl("get", "pods",
		"--namespace", namespaceName,
		"--selector", fmt.Sprintf("app.kubernetes.io/component=application,app.kubernetes.io/name=%s, app.kubernetes.io/part-of=%s", appName, namespaceName),
		"-o", "jsonpath="+jsonPath)
	Expect(err).NotTo(HaveOccurred())

	return strings.Split(out, "\n")
}

func (m *Machine) GetSettingsFrom(location string) (*settings.Settings, error) {
	return settings.LoadFrom(location)
}

func (m *Machine) GetSettings() (*settings.Settings, error) {
	return settings.Load()
}

// SaveApplicationSpec is a debugging helper function saving the
// specified application's manifest and helm data (values, chart) into
// file and directory named after the application.
//
// Note: Intended use is debugging of a test case. Most tests cases
// will not need this as part of their regular operation.
//
// __Attention__: The created files and directory are __not cleaned up automatically__
func (m *Machine) SaveApplicationSpec(appName string) {

	out, err := m.Epinio("", "app", "manifest", appName,
		filepath.Join(m.nodeTmpDir, appName+"-manifest.yaml"))
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio("", "app", "export",
		filepath.Join(m.nodeTmpDir, appName, appName+"-helm"))
	Expect(err).ToNot(HaveOccurred(), out)
}

// SaveServerLogs is a debugging helper function saving the
// test server's logs (last 3 minutes) into the specified file.
//
// Note: Intended use is debugging of a test case. Most tests cases
// will not need this as part of their regular operation.
//
// __Attention__: The created file is __not cleaned up automatically__
func (m *Machine) SaveServerLogs(destination string) {
	// Locate the server pod
	serverPodName, err := proc.Kubectl("get", "pod",
		"-n", "epinio",
		"-l", "app.kubernetes.io/component=epinio-server",
		"-o", "name")
	Expect(err).ToNot(HaveOccurred(), serverPodName)
	serverPodName = strings.TrimSpace(serverPodName)

	// Get server logs of the last 3 minutes
	log, err := proc.Kubectl("logs",
		"-c", "epinio-server",
		"-n", "epinio",
		"--since", "3m",
		serverPodName)
	Expect(err).ToNot(HaveOccurred(), log)

	// And save them into the specified file
	err = os.WriteFile(filepath.Join(m.nodeTmpDir, destination), []byte(log), 0600)
	Expect(err).ToNot(HaveOccurred())
}
