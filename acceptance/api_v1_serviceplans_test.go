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

var _ = Describe("ServicePlans API Application Endpoints", func() {

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

	Context("ServicePlans", func() {

		BeforeEach(func() {
			setupInClusterServices()
		})

		Describe("GET /api/v1/orgs/:org/serviceclasses/:serviceclass/serviceplans", func() {
			It("lists all serviceplans in the org", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/%s/serviceclasses/%s/serviceplans",
					serverURL, org, "mariadb"), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				var data services.ServicePlanList
				err = json.Unmarshal(bodyBytes, &data)
				Expect(err).ToNot(HaveOccurred())
				Expect(data[0].Name).To(Equal("10-1-26"))
				Expect(data[0].Description).To(ContainSubstring("MariaDB Server is intended for mission-critical"))
				Expect(data[0].Free).To(BeTrue())
				Expect(data[1].Name).To(Equal("10-1-28"))
				Expect(data[1].Description).To(ContainSubstring("MariaDB Server is intended for mission-critical"))
				Expect(data[1].Free).To(BeTrue())
			})

			It("returns a 404 when the org does not exist", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs/:org/idontexist/:serviceclass/serviceplans", serverURL), strings.NewReader(""))
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
