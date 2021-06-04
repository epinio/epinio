package acceptance_test

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/epinio/epinio/helpers"
)

// This file provides a number of utility functions encapsulating often-used sequences.
// I.e. create/delete applications/services, bind/unbind services, etc.
// This is done in the hope of enhancing the readability of various before/after blocks.

func newOrgName() string {
	return "orgs-" + strconv.Itoa(int(time.Now().Nanosecond()))
}

func newAppName() string {
	return "apps-" + strconv.Itoa(int(time.Now().Nanosecond()))
}

func newServiceName() string {
	return "service-" + strconv.Itoa(int(time.Now().Nanosecond()))
}

// func Curl is used to make requests against a server
func Curl(method, uri string, requestBody *strings.Reader) (*http.Response, error) {
	request, err := http.NewRequest(method, uri, requestBody)
	request.SetBasicAuth(epinioUser, epinioPassword)
	if err != nil {
		return nil, err
	}
	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // self signed certs
	}
	return (&http.Client{Transport: transCfg}).Do(request)
}

func setupAndTargetOrg(org string) {
	By("creating an org")

	out, err := Epinio("org create "+org, "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	orgs, err := Epinio("org list", "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	ExpectWithOffset(1, orgs).To(MatchRegexp(org))

	By("targeting an org")

	out, err = Epinio(fmt.Sprintf("target %s", org), nodeTmpDir)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	out, err = Epinio("target", nodeTmpDir)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp("Currently targeted organization: " + org))
}

func setupInClusterServices() {
	out, err := RunProc("../dist/epinio-linux-amd64 enable services-incluster", "", false)
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// Wait until classes appear
	EventuallyWithOffset(1, func() error {
		_, err = helpers.Kubectl("get clusterserviceclass mariadb")
		return err
	}, "5m").ShouldNot(HaveOccurred())

	// Wait until plans appear
	EventuallyWithOffset(1, func() error {
		_, err = helpers.Kubectl("get clusterserviceplan mariadb-10-3-22")
		return err
	}, "5m").ShouldNot(HaveOccurred())
}

func makeApp(appName string, instances int, deployFromCurrentDir bool) string {
	currentDir, err := os.Getwd()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	appDir := path.Join(currentDir, "../assets/sample-app")

	return makeAppWithDir(appName, instances, deployFromCurrentDir, appDir)
}

func makeGolangApp(appName string, instances int, deployFromCurrentDir bool) string {
	currentDir, err := os.Getwd()
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	appDir := path.Join(currentDir, "../assets/golang-sample-app")

	return makeAppWithDir(appName, instances, deployFromCurrentDir, appDir)
}

func makeAppWithDir(appName string, instances int, deployFromCurrentDir bool, appDir string) string {
	var pushOutput string
	var err error
	if deployFromCurrentDir {
		// Note: appDir is handed to the working dir argument of Epinio().
		// This means that the command runs with it as the CWD.
		pushOutput, err = Epinio(fmt.Sprintf("apps push %s --instances %d", appName, instances), appDir)
	} else {
		// Note: appDir is handed as second argument to the epinio cli.
		// This means that the command gets the sources from that directory instead of CWD.
		pushOutput, err = Epinio(fmt.Sprintf("apps push %s %s --instances %d", appName, appDir, instances), "")
	}
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), pushOutput)

	// And check presence

	out, err := Epinio("app list", "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp(fmt.Sprintf(`%s.*\|.*%d\/%d.*\|.*`, appName, instances, instances)))

	return pushOutput
}

func makeCustomService(serviceName string) {
	out, err := Epinio(fmt.Sprintf("service create-custom %s username epinio-user", serviceName), "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check presence

	out, err = Epinio("service list", "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp(serviceName))
}

func makeCatalogService(serviceName string) {
	out, err := Epinio(fmt.Sprintf("service create %s mariadb 10-3-22", serviceName), "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// Look for the messaging indicating that the command waited
	ExpectWithOffset(1, out).To(MatchRegexp("Provisioning"))
	ExpectWithOffset(1, out).To(MatchRegexp("Service Provisioned"))

	// Check presence

	out, err = Epinio("service list", "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp(serviceName))
}

func makeCatalogServiceDontWait(serviceName string) {
	out, err := Epinio(fmt.Sprintf("service create --dont-wait %s mariadb 10-3-22", serviceName), "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// Look for indicator that command did not wait
	ExpectWithOffset(1, out).To(MatchRegexp("to watch when it is provisioned"))

	// Check presence

	out, err = Epinio("service list", "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp(serviceName))

	// And explicitly wait for it being provisioned

	EventuallyWithOffset(1, func() string {
		out, err = Epinio("service show "+serviceName, "")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "5m").Should(MatchRegexp(`Status .*\|.* Provisioned`))
}

func bindAppService(appName, serviceName, org string) {
	out, err := Epinio(fmt.Sprintf("service bind %s %s", serviceName, appName), "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check deep into the kube structures
	verifyAppServiceBound(appName, serviceName, org, 2)
}

func verifyAppServiceBound(appName, serviceName, org string, offset int) {
	out, err := helpers.Kubectl(fmt.Sprintf("get deployment -n %s %s -o=jsonpath='{.spec.template.spec.volumes}'", org, appName))
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).To(MatchRegexp(serviceName))

	out, err = helpers.Kubectl(fmt.Sprintf("get deployment -n %s %s -o=jsonpath='{.spec.template.spec.containers[0].volumeMounts}'", org, appName))
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).To(MatchRegexp("/services/" + serviceName))
}

func deleteApp(appName string) {
	out, err := Epinio("app delete "+appName, "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	// TODO: Fix `epinio delete` from returning before the app is deleted #131

	EventuallyWithOffset(1, func() string {
		out, err := Epinio("app list", "")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "1m").ShouldNot(MatchRegexp(`.*%s.*`, appName))
}

func cleanupApp(appName string) {
	out, err := Epinio("app delete "+appName, "")
	// TODO: Fix `epinio delete` from returning before the app is deleted #131

	if err != nil {
		fmt.Printf("deleting app failed : %s\n%s", err.Error(), out)
	}
}

func deleteService(serviceName string) {
	out, err := Epinio("service delete "+serviceName, "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And check non-presence
	EventuallyWithOffset(1, func() string {
		out, err = Epinio("service list", "")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "10m").ShouldNot(MatchRegexp(serviceName))
}

func cleanupService(serviceName string) {
	out, err := Epinio("service delete "+serviceName, "")

	if err != nil {
		fmt.Printf("deleting service failed : %s\n%s", err.Error(), out)
	}
}

func unbindAppService(appName, serviceName, org string) {
	out, err := Epinio(fmt.Sprintf("service unbind %s %s", serviceName, appName), "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)

	// And deep check in kube structures for non-presence
	verifyAppServiceNotbound(appName, serviceName, org, 2)
}

func cleanUnbindAppService(appName, serviceName, org string) {
	out, err := Epinio(fmt.Sprintf("service unbind %s %s", serviceName, appName), "")
	if err != nil {
		fmt.Printf("unbinding service failed: %s\n%s", err.Error(), out)
	}
}

func verifyAppServiceNotbound(appName, serviceName, org string, offset int) {
	out, err := helpers.Kubectl(fmt.Sprintf("get deployment -n %s %s -o=jsonpath='{.spec.template.spec.volumes}'", org, appName))
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).ToNot(MatchRegexp(serviceName))

	out, err = helpers.Kubectl(fmt.Sprintf("get deployment -n %s %s -o=jsonpath='{.spec.template.spec.containers[0].volumeMounts}'", org, appName))
	ExpectWithOffset(offset, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(offset, out).ToNot(MatchRegexp("/services/" + serviceName))
}

func verifyOrgNotExist(org string) {
	out, err := Epinio("org list", "")
	ExpectWithOffset(1, err).ToNot(HaveOccurred(), out)
	ExpectWithOffset(1, out).ToNot(MatchRegexp(org))
}

func makeWebSocketConnection(url string) *websocket.Conn {
	headers := http.Header{
		"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", epinioUser, epinioPassword)))},
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

func getPodNames(appName, orgName string) []string {
	jsonPath := `'{range .items[*]}{.metadata.name}{"\n"}{end}'`
	out, err := helpers.Kubectl(fmt.Sprintf("get pods -n %s --selector 'app.kubernetes.io/component=application,app.kubernetes.io/name=%s, app.kubernetes.io/part-of=%s' -o=jsonpath=%s", orgName, appName, orgName, jsonPath))
	Expect(err).NotTo(HaveOccurred())

	return strings.Split(out, "\n")
}
