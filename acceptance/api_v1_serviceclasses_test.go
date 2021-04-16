package acceptance_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/internal/services"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceClasses API Application Endpoints", func() {

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

	Context("ServiceClasses", func() {

		BeforeEach(func() {
			setupInClusterServices()
		})

		Describe("GET /api/v1/orgs/:org/serviceclasses", func() {
			It("lists all serviceclasses in the org", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/%s/serviceclasses",
					serverURL, org), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				var data services.ServiceClassList
				err = json.Unmarshal(bodyBytes, &data)
				Expect(err).ToNot(HaveOccurred())
				Expect(data[0].Name).To(Equal("mariadb"))
				Expect(data[0].Description).To(ContainSubstring("Helm Chart for mariadb"))
				Expect(data[0].Broker).To(Equal("minibroker"))
				Expect(data[1].Name).To(Equal("mongodb"))
				Expect(data[1].Description).To(ContainSubstring("Helm Chart for mongodb"))
				Expect(data[1].Broker).To(Equal("minibroker"))
			})

			It("returns a 404 when the org does not exist", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/idontexist/serviceclasses", serverURL), strings.NewReader(""))
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
