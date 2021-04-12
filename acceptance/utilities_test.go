package acceptance_test

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

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
	if err != nil {
		return nil, err
	}
	return (&http.Client{}).Do(request)
}

func setupAndTargetOrg(org string) {
	By("creating an org")

	out, err := Epinio("org create "+org, "")
	Expect(err).ToNot(HaveOccurred(), out)

	orgs, err := Epinio("org list", "")
	Expect(err).ToNot(HaveOccurred())
	Expect(orgs).To(MatchRegexp(org))

	By("targeting an org")

	out, err = Epinio("target "+org, "")
	Expect(err).ToNot(HaveOccurred(), out)

	out, err = Epinio("target", "")
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).To(MatchRegexp("Currently targeted organization: " + org))
}

func setupInClusterServices() {
	out, err := Epinio("enable services-incluster", "")
	Expect(err).ToNot(HaveOccurred(), out)

	// Wait until classes appear
	Eventually(func() error {
		_, err = helpers.Kubectl("get clusterserviceclass mariadb")
		return err
	}, "5m").ShouldNot(HaveOccurred())

	// Wait until plans appear
	Eventually(func() error {
		_, err = helpers.Kubectl("get clusterserviceplan mariadb-10-3-22")
		return err
	}, "5m").ShouldNot(HaveOccurred())
}

func makeApp(appName string) {
	currentDir, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	appDir := path.Join(currentDir, "../sample-app")

	out, err := Epinio(fmt.Sprintf("apps push %s --verbosity 1", appName), appDir)
	Expect(err).ToNot(HaveOccurred(), out)

	// And check presence

	out, err = Epinio("app list", "")
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).To(MatchRegexp(appName + `.*\|.*1\/1.*\|.*`))
}

func makeCustomService(serviceName string) {
	out, err := Epinio(fmt.Sprintf("service create-custom %s username epinio-user", serviceName), "")
	Expect(err).ToNot(HaveOccurred(), out)

	// And check presence

	out, err = Epinio("service list", "")
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).To(MatchRegexp(serviceName))
}

func makeCatalogService(serviceName string) {
	out, err := Epinio(fmt.Sprintf("service create %s mariadb 10-3-22", serviceName), "")
	Expect(err).ToNot(HaveOccurred(), out)

	// Look for the messaging indicating that the command waited
	Expect(out).To(MatchRegexp("Provisioning"))
	Expect(out).To(MatchRegexp("Service Provisioned"))

	// Check presence

	out, err = Epinio("service list", "")
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).To(MatchRegexp(serviceName))
}

func makeCatalogServiceDontWait(serviceName string) {
	out, err := Epinio(fmt.Sprintf("service create --dont-wait %s mariadb 10-3-22", serviceName), "")
	Expect(err).ToNot(HaveOccurred(), out)

	// Look for indicator that command did not wait
	Expect(out).To(MatchRegexp("to watch when it is provisioned"))

	// Check presence

	out, err = Epinio("service list", "")
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).To(MatchRegexp(serviceName))

	// And explicitly wait for it being provisioned

	Eventually(func() string {
		out, err = Epinio("service show "+serviceName, "")
		Expect(err).ToNot(HaveOccurred(), out)
		return out
	}, "5m").Should(MatchRegexp(`Status .*\|.* Provisioned`))
}

func bindAppService(appName, serviceName, org string) {
	out, err := Epinio(fmt.Sprintf("service bind %s %s", serviceName, appName), "")
	Expect(err).ToNot(HaveOccurred(), out)

	// And check deep into the kube structures
	verifyAppServiceBound(appName, serviceName, org)
}

func verifyAppServiceBound(appName, serviceName, org string) {
	out, err := helpers.Kubectl(fmt.Sprintf("get deployment -n epinio-workloads %s.%s -o=jsonpath='{.spec.template.spec.volumes}'", org, appName))
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).To(MatchRegexp(serviceName))

	out, err = helpers.Kubectl(fmt.Sprintf("get deployment -n epinio-workloads %s.%s -o=jsonpath='{.spec.template.spec.containers[0].volumeMounts}'", org, appName))
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).To(MatchRegexp("/services/" + serviceName))
}

func deleteApp(appName string) {
	out, err := Epinio("app delete "+appName, "")
	Expect(err).ToNot(HaveOccurred(), out)
	// TODO: Fix `epinio delete` from returning before the app is deleted #131

	Eventually(func() string {
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
	Expect(err).ToNot(HaveOccurred(), out)

	// And check non-presence
	Eventually(func() string {
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
	Expect(err).ToNot(HaveOccurred(), out)

	// And deep check in kube structures for non-presence
	verifyAppServiceNotbound(appName, serviceName, org)
}

func cleanUnbindAppService(appName, serviceName, org string) {
	out, err := Epinio(fmt.Sprintf("service unbind %s %s", serviceName, appName), "")
	if err != nil {
		fmt.Printf("unbinding service failed: %s\n%s", err.Error(), out)
	}
}

func verifyAppServiceNotbound(appName, serviceName, org string) {
	out, err := helpers.Kubectl(fmt.Sprintf("get deployment -n epinio-workloads %s.%s -o=jsonpath='{.spec.template.spec.volumes}'", org, appName))
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).ToNot(MatchRegexp(serviceName))

	out, err = helpers.Kubectl(fmt.Sprintf("get deployment -n epinio-workloads %s.%s -o=jsonpath='{.spec.template.spec.containers[0].volumeMounts}'", org, appName))
	Expect(err).ToNot(HaveOccurred(), out)
	Expect(out).ToNot(MatchRegexp("/services/" + serviceName))
}
