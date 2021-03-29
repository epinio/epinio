package acceptance_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/codeskyblue/kexec"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
)

var _ = Describe("api/v1/org/:org/applications", func() {
	var org = newOrgName()
	var serverProcess *kexec.KCommand
	var err error
	var serverPort = 8080 + config.GinkgoConfig.ParallelNode
	var serverURL = fmt.Sprintf("http://127.0.0.1:%d", serverPort)
	var app1, app2 string

	BeforeEach(func() {
		setupAndTargetOrg(org)
		serverProcess, err = startCarrierServer(serverPort)
		Expect(err).ToNot(HaveOccurred())

		app1 = newAppName()
		app2 = newAppName()
		makeApp(app1)
		makeApp(app2)

		// Wait for server to be up and running
		Eventually(func() error {
			_, err := Curl(serverURL+"/api/v1/info", strings.NewReader(""))
			return err
		}, "1m").ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(serverProcess.Process.Kill()).ToNot(HaveOccurred())
	})

	It("lists all applications belonging to the org", func() {
		response, err := Curl(fmt.Sprintf("%s/api/v1/org/%s/applications", serverURL, org), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		Expect(response.StatusCode).To(Equal(http.StatusOK))
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(bodyBytes).To(MatchRegexp(app1))
		Expect(bodyBytes).To(MatchRegexp(app2))
	})
})
