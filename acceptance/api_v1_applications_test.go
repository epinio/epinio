package acceptance_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/codeskyblue/kexec"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/suse/carrier/internal/application"
)

var _ = Describe("API Application Endpoints", func() {

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

	Describe("GET api/v1/org/:org/applications", func() {
		It("lists all applications belonging to the org", func() {
			response, err := Curl(fmt.Sprintf("%s/api/v1/org/%s/applications", serverURL, org), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			var apps application.ApplicationList
			err = json.Unmarshal(bodyBytes, &apps)
			Expect(err).ToNot(HaveOccurred())
			Expect(apps[0].Name).To(Equal(app1))
			Expect(apps[0].Organization).To(Equal(org))
			Expect(apps[0].Status).To(Equal("1/1"))
			Expect(apps[1].Name).To(Equal(app2))
			Expect(apps[1].Organization).To(Equal(org))
			Expect(apps[1].Status).To(Equal("1/1"))
		})

		It("returns a 404 when the org does not exist", func() {
			response, err := Curl(fmt.Sprintf("%s/api/v1/org/idontexist/applications", serverURL), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	Describe("GET api/v1/org/:org/applications/:app", func() {
		It("lists the application data", func() {
			response, err := Curl(fmt.Sprintf("%s/api/v1/org/%s/applications/%s", serverURL, org, app1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(bodyBytes).To(MatchRegexp(app1))

			var app application.Application
			err = json.Unmarshal(bodyBytes, &app)
			Expect(err).ToNot(HaveOccurred())
			Expect(app.Name).To(Equal(app1))
			Expect(app.Organization).To(Equal(org))
			Expect(app.Status).To(Equal("1/1"))
		})

		It("returns a 404 when the org does not exist", func() {
			response, err := Curl(fmt.Sprintf("%s/api/v1/org/idontexist/applications/%s", serverURL, app1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound))
		})
	})
})
