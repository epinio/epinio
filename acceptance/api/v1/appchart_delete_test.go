package v1_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	v1 "github.com/epinio/epinio/internal/api/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ChartDelete Endpoint", func() {

	It("fails to delete an unknown app chart", func() {
		response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/appcharts/bogus",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound))

		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(bodyBytes)).To(Equal(`{"errors":[{"status":404,"title":"Application Chart 'bogus' does not exist","details":""}]}`))
	})

	When("deleting an existing chart chart", func() {
		BeforeEach(func() {
			out, err := env.Epinio("", "apps", "chart", "create", "to.be.deleted", "wolf")
			Expect(err).ToNot(HaveOccurred(), out)
		})

		It("deletes the chart", func() {
			response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/appcharts/to.be.deleted",
				serverURL, v1.Root), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(bodyBytes)).To(Equal(`{"status":"ok"}`))
		})
	})
})
