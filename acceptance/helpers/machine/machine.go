// Package machine provides a number of utility functions encapsulating often-used sequences.
// I.e. create/delete applications/services, bind/unbind services, etc.
// This is done in the hope of enhancing the readability of various before/after blocks.
package machine

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	urlpkg "net/url"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/acceptance/helpers/proc"
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

func (m *Machine) OnStageFailureShowStagingLogs(err error, out, app string) {
	if err == nil || !strings.Contains(out, "Failed to stage") {
		return
	}

	m.ShowStagingLogs(app)
}

func (m *Machine) ShowStagingLogs(app string) {
	namespace := m.GetNamespace()
	_, _ = proc.Run("", true, "bash", "-c", fmt.Sprintf(`
app="%s"
namespace="%s"
sns=epinio-staging

echo ""
echo ___ XXXX APP "($app)" SPACE "($namespace)" ___
echo ___ LOGS _ _ __ ___ _____ ________ _____________
for pod in $(kubectl get pods -n "${sns}" -l "app.kubernetes.io/component=staging,app.kubernetes.io/part-of=${namespace},app.kubernetes.io/name=${app}" -o 'jsonpath={.items[*].metadata.name}')
do
    for container in $(kubectl get pods -n "${sns}" "${pod}" -o jsonpath='{.spec.initContainers[*].name}')
    do
	case ${container} in
	    *linkerd*) continue ;;
	esac
	echo ___ INIT POD "($pod)" C "($container)" _ _ __ ___ _____ ________ _____________
	kubectl logs -n "${sns}" -c "${container}" "${pod}"
    done
    for container in $(kubectl get pods -n "${sns}" "${pod}" -o jsonpath='{.spec.containers[*].name}')
    do
	case ${container} in
	    *linkerd*) continue ;;
	esac
	echo ___ RUNC POD "($pod)" C "($container)" _ _ __ ___ _____ ________ _____________
	kubectl logs -n "${sns}" -c "${container}" "${pod}"
    done
done
echo ___ LOGS _ _ __ ___ _____ ________ _____________
echo ""
`, app, namespace))
}

// Epinio invokes the `epinio` binary, running the specified command.
// It returns the command output and/or error.
// dir parameter defines the directory from which the command should be run.
// It defaults to the current dir if left empty.
func (m *Machine) Epinio(dir, command string, arg ...string) (string, error) {
	return proc.Run(dir, false, m.epinioBinaryPath, append([]string{command}, arg...)...)
}

func (m *Machine) SetupAndTargetNamespace(namespace string) {
	By("creating a namespace")

	out, err := m.Epinio("", "namespace", "create", namespace)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio("", "namespace", "show", namespace)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, out).To(MatchRegexp("Name.*|.*" + namespace))

	m.TargetNamespace(namespace)
}

func (m *Machine) GetNamespace() string {
	out, err := m.Epinio(m.nodeTmpDir, "target")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio(m.nodeTmpDir, "target")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// Extract namespace from command response.
	// Brittle. Only way we have at the moment.
	// Will also become superfluous when staging logs can be extracted directly.
	// Note: len-2 because the last line (len-1) is empty.
	lines := strings.Split(out, "\n")
	space := strings.TrimPrefix(lines[len(lines)-2], "Currently targeted namespace: ")
	return space
}

func (m *Machine) TargetNamespace(namespace string) {
	By("targeting a namespace")

	out, err := m.Epinio(m.nodeTmpDir, "target", namespace)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio(m.nodeTmpDir, "target")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp("Currently targeted namespace: " + namespace))
}

func (m *Machine) DeleteNamespace(namespace string) {
	By("deleting a namespace")

	out, err := m.Epinio("", "namespace", "delete", "-f", namespace)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = m.Epinio("", "namespace", "show", namespace)
	ExpectWithOffset(1, err).To(HaveOccurred())
	ExpectWithOffset(1, out).To(MatchRegexp(".*Not Found: Targeted namespace.*does not exist.*"))
}

func (m *Machine) VerifyNamespaceNotExist(namespace string) {
	out, err := m.Epinio("", "namespace", "list")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).ToNot(MatchRegexp(namespace))
}

func (m *Machine) MakeWebSocketConnection(authToken string, url string, subprotocols ...string) *websocket.Conn {
	u, err := urlpkg.Parse(url)
	Expect(err).NotTo(HaveOccurred(), url)
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

	Expect(err).NotTo(HaveOccurred(), string(b))
	Expect(response.StatusCode).To(Equal(http.StatusSwitchingProtocols))
	return ws
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

func (m *Machine) GetConfigFrom(location string) (*config.Config, error) {
	return config.LoadFrom(location)
}

func (m *Machine) GetConfig() (*config.Config, error) {
	return config.Load()
}
