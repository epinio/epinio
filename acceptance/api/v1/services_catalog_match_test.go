package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceCatalog Match Endpoint", func() {
	It("lists all catalog entries for empty prefix", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/catalogservicesmatches",
			serverURL, v1.Root),
			strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		resp := models.CatalogMatchResponse{}
		err = json.Unmarshal(bodyBytes, &resp)
		Expect(err).ToNot(HaveOccurred())

		// See global BeforeEach for where this namespace is set up.
		Expect(resp.Names).Should(ContainElements(
			"mysql-dev",
			"postgresql-dev",
			"rabbitmq-dev",
			"redis-dev",
		))
	})

	It("lists no catalog entries matching the prefix", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/catalogservicesmatches/bogus",
			serverURL, v1.Root),
			strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		resp := models.CatalogMatchResponse{}
		err = json.Unmarshal(bodyBytes, &resp)
		Expect(err).ToNot(HaveOccurred())

		// See global BeforeEach for where this namespace is set up.
		Expect(resp.Names).Should(BeEmpty())
	})

	It("lists all catalog entries matching the prefix", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/catalogservicesmatches/r",
			serverURL, v1.Root),
			strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		resp := models.CatalogMatchResponse{}
		err = json.Unmarshal(bodyBytes, &resp)
		Expect(err).ToNot(HaveOccurred())

		// See global BeforeEach for where this namespace is set up.
		Expect(resp.Names).ShouldNot(BeEmpty())
		Expect(resp.Names).Should(ContainElements(
			"rabbitmq-dev",
			"redis-dev",
		))
	})

	When("basic auth credentials are not provided", func() {
		It("returns a 401 response", func() {
			request, err := http.NewRequest("GET", fmt.Sprintf("%s%s/catalogservicesmatches",
				serverURL, v1.Root), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			response, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
		})
	})
})
