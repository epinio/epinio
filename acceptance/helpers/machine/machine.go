// Package machine provides a number of utility functions encapsulating often-used sequences.
// I.e. create/delete applications/configurations, bind/unbind configurations, etc.
// This is done in the hope of enhancing the readability of various before/after blocks.
package machine

import (
	"crypto/tls"
	"fmt"
	"io"
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
	user             string
	password         string
	adminToken       string
	userToken        string
	root             string
	epinioBinaryPath string
}

func New(dir, user, password, adminToken, userToken, root, epinioBinaryPath string) Machine {
	return Machine{
		nodeTmpDir:       dir,
		user:             user,
		password:         password,
		adminToken:       adminToken,
		userToken:        userToken,
		root:             root,
		epinioBinaryPath: epinioBinaryPath,
	}
}

func (m *Machine) ShowStagingLogs(app string) {
	_ = m.Epinio("", app, "app", "logs", "--staging", app)
}

// Epinio invokes the `epinio` binary, running the specified command.
// It returns the command output and/or error.
// dir parameter defines the directory from which the command should be run.
// It defaults to the current dir if left empty.
func (m *Machine) Epinio(dir, command string, arg ...string) string {
	out, err := proc.Run(dir, false, m.epinioBinaryPath, append([]string{command}, arg...)...)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	return out
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

	_ = m.Epinio("", "namespace", "create", namespace)
	out := m.Epinio("", "namespace", "show", namespace)
	ExpectWithOffset(1, out).To(MatchRegexp("Name.*|.*" + namespace))
}

func (m *Machine) TargetNamespace(namespace string) {
	By(fmt.Sprintf("targeting a namespace: %s", namespace))

	_ = m.Epinio(m.nodeTmpDir, "target", namespace)
	out := m.Epinio(m.nodeTmpDir, "target")
	ExpectWithOffset(1, out).To(MatchRegexp("Currently targeted namespace: " + namespace))
}

func (m *Machine) SetupAndTargetNamespace(namespace string) {
	m.SetupNamespace(namespace)
	m.TargetNamespace(namespace)
}

func (m *Machine) DeleteNamespace(namespace string) {
	By(fmt.Sprintf("deleting a namespace: %s", namespace))

	_ = m.Epinio("", "namespace", "delete", "-f", namespace)
	out := m.Epinio("", "namespace", "show", namespace)
	ExpectWithOffset(1, out).To(MatchRegexp(".*Not Found: namespace '" + namespace + "' does not exist.*"))
}

func (m *Machine) VerifyNamespaceNotExist(namespace string) {
	EventuallyWithOffset(1,
		m.Epinio("", "namespace", "list"),
		"2m",
	).ShouldNot(MatchRegexp(namespace))
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
		b, _ = io.ReadAll(response.Body)
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
	manifestPath := filepath.Join(m.nodeTmpDir, appName+"-manifest.yaml")
	_ = m.Epinio("", "app", "manifest", appName, manifestPath)

	helmPath := filepath.Join(m.nodeTmpDir, appName, appName+"-helm")
	_ = m.Epinio("", "app", "export", helmPath)
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
