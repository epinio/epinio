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

var _ = Describe("Orgs API Application Endpoints", func() {
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
	Context("Orgs", func() {
		Describe("GET api/v1/orgs", func() {
			It("lists all organizations", func() {
				response, err := Curl("GET", fmt.Sprintf("%s/api/v1/orgs", serverURL),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				var orgs []string
				err = json.Unmarshal(bodyBytes, &orgs)
				Expect(err).ToNot(HaveOccurred())

				// See global BeforeEach for where this org is set up.
				Expect(orgs).Should(ContainElements(org))
			})
		})

		Describe("POST api/v1/orgs", func() {
			It("fails for non JSON body", func() {
				response, err := Curl("POST", fmt.Sprintf("%s/api/v1/orgs", serverURL),
					strings.NewReader(``))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal("unexpected end of JSON input\n"))
			})

			It("fails for non-object JSON body", func() {
				response, err := Curl("POST", fmt.Sprintf("%s/api/v1/orgs", serverURL),
					strings.NewReader(`[]`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal("json: cannot unmarshal array into Go value of type map[string]string\n"))
			})

			It("fails for JSON object without name key", func() {
				response, err := Curl("POST", fmt.Sprintf("%s/api/v1/orgs", serverURL),
					strings.NewReader(`{}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal("Name of organization to create not found\n"))
			})

			It("fails for a known organization", func() {
				// Create the org
				response, err := Curl("POST", fmt.Sprintf("%s/api/v1/orgs", serverURL),
					strings.NewReader(`{"name":"birdy"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(""))

				// And the 2nd attempt should now fail
				By("creating the same org a second time")

				response, err = Curl("POST", fmt.Sprintf("%s/api/v1/orgs", serverURL),
					strings.NewReader(`{"name":"birdy"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err = ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusConflict), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal("Organization 'birdy' already exists\n"))
			})

			It("creates a new organization", func() {
				response, err := Curl("POST", fmt.Sprintf("%s/api/v1/orgs", serverURL),
					strings.NewReader(`{"name":"birdwatcher"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(""))
			})
		})
	})
})
