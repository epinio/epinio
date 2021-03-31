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
			_, err := Curl("GET", serverURL+"/api/v1/info", strings.NewReader(""))
			return err
		}, "1m").ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(serverProcess.Process.Kill()).ToNot(HaveOccurred())
	})

	Describe("GET api/v1/org/:org/applications", func() {
		AfterEach(func() {
			deleteApp(app1)
			deleteApp(app2)
		})

		It("lists all applications belonging to the org", func() {
			response, err := Curl("GET", fmt.Sprintf("%s/api/v1/org/%s/applications", serverURL, org), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

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
			response, err := Curl("GET", fmt.Sprintf("%s/api/v1/org/idontexist/applications", serverURL), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	Describe("GET api/v1/org/:org/applications/:app", func() {
		AfterEach(func() {
			deleteApp(app1)
			deleteApp(app2)
		})

		It("lists the application data", func() {
			response, err := Curl("GET", fmt.Sprintf("%s/api/v1/org/%s/applications/%s", serverURL, org, app1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			var app application.Application
			err = json.Unmarshal(bodyBytes, &app)
			Expect(err).ToNot(HaveOccurred())
			Expect(app.Name).To(Equal(app1))
			Expect(app.Organization).To(Equal(org))
			Expect(app.Status).To(Equal("1/1"))
		})

		It("returns a 404 when the org does not exist", func() {
			response, err := Curl("GET", fmt.Sprintf("%s/api/v1/org/idontexist/applications/%s", serverURL, app1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})

		It("returns a 404 when the app does not exist", func() {
			response, err := Curl("GET", fmt.Sprintf("%s/api/v1/org/%s/applications/bogus", serverURL, org), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	Describe("DELETE api/v1/org/:org/applications/:app", func() {
		var service string

		BeforeEach(func() {
			service = newServiceName()
			makeCustomService(service)
			bindAppService(app1, service, org)
		})

		AfterEach(func() {
			deleteApp(app2) // This one was not deleted in the test
			cleanupService(service)
		})

		It("removes the application, unbinds bound services", func() {
			response, err := Curl("DELETE", fmt.Sprintf("%s/api/v1/org/%s/applications/%s", serverURL, org, app1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			var resp map[string][]string
			err = json.Unmarshal(bodyBytes, &resp)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp).To(HaveLen(1))
			Expect(resp).To(HaveKey("UnboundServices"))
			Expect(resp["UnboundServices"]).To(ContainElement(service))
		})

		It("returns a 404 when the org does not exist", func() {
			response, err := Curl("DELETE", fmt.Sprintf("%s/api/v1/org/idontexist/applications/%s", serverURL, app1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})

		It("returns a 404 when the app does not exist", func() {
			response, err := Curl("DELETE", fmt.Sprintf("%s/api/v1/org/%s/applications/bogus", serverURL, org), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})
})
