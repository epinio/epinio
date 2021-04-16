package acceptance_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Services API Application Endpoints", func() {

	var org string

	BeforeEach(func() {
		org = newOrgName()
		setupAndTargetOrg(org)

		// Wait for server to be up and running
		Eventually(func() error {
			_, err := Curl("GET", serverURL+"/api/v1/info", strings.NewReader(""))
			return err
		}, "1m").ShouldNot(HaveOccurred())
	})

	Context("Services", func() {
		var svc1, svc2 string

		BeforeEach(func() {
			svc1 = newServiceName()
			svc2 = newServiceName()
			setupInClusterServices()
			makeCatalogService(svc1)
			makeCustomService(svc2)
		})

		Describe("GET /api/v1/orgs/:org/services", func() {
			AfterEach(func() {
				deleteService(svc1)
				deleteService(svc2)
			})

			It("lists all services in the org", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/%s/services",
					serverURL, org), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				var data map[string]interface{}
				err = json.Unmarshal(bodyBytes, &data)
				Expect(err).ToNot(HaveOccurred())

				orgServices := data["Services"].([]interface{})
				Expect(err).ToNot(HaveOccurred())
				serviceNames := []string{}
				serviceNames = append(serviceNames, orgServices[0].(map[string]interface{})["Service"].(string))
				serviceNames = append(serviceNames, orgServices[1].(map[string]interface{})["Service"].(string))
				Expect(serviceNames).Should(ContainElements(svc1, svc2))
			})

			It("returns a 404 when the org does not exist", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/idontexist/services", serverURL), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})
		})

		Describe("GET api/v1/orgs/:org/services/:service", func() {
			AfterEach(func() {
				deleteService(svc1)
				deleteService(svc2)
			})

			It("lists the service data", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/%s/services/%s", serverURL, org, svc1), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())

				var service map[string]string
				err = json.Unmarshal(bodyBytes, &service)
				Expect(err).ToNot(HaveOccurred())
				Expect(service["Class"]).To(Equal("mariadb"))
				Expect(service["Status"]).To(Equal("Provisioned"))
				Expect(service["Plan"]).To(Equal("10-3-22"))
			})

			It("returns a 404 when the org does not exist", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/idontexist/services/%s", serverURL, svc1), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})

			It("returns a 404 when the service does not exist", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/%s/services/bogus", serverURL, org), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			})
		})
	})
})
