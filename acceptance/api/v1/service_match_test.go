package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceMatch Endpoint", func() {
	var namespace string

	When("namespace doesn't exist", func() {
		It("returns an error", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/doesntexist/servicesmatches",
				serverURL, v1.Root)
			response, err := env.Curl("POST", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusMethodNotAllowed))
		})
	})

	When("namespace exists", func() {
		var serviceName string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			serviceName = catalog.NewServiceName()

			By("create it")
			out, err := env.Epinio("", "service", "create", "mysql-dev", serviceName)
			Expect(err).ToNot(HaveOccurred(), out)

			By("show it")
			out, err = env.Epinio("", "service", "show", serviceName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp(fmt.Sprintf("Name.*\\|.*%s", serviceName)))

			By("wait for deployment")
			Eventually(func() string {
				out, _ := env.Epinio("", "service", "show", serviceName)
				return out
			}, "2m", "5s").Should(MatchRegexp("Status.*\\|.*deployed"))

			By(fmt.Sprintf("%s/%s up", namespace, serviceName))

		})

		AfterEach(func() {
			// out, err := env.Epinio("", "service", "delete", service)
			// Expect(err).ToNot(HaveOccurred(), out)

			// Eventually(func() string {
			// 	out, _ := env.Epinio("", "service", "delete", service)
			// 	return out
			// }, "1m", "5s").Should(MatchRegexp("service not found"))

			env.DeleteNamespace(namespace)
		})

		It("lists all services for empty prefix", func() {
			By("querying matches")
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/servicesmatches",
				serverURL, v1.Root, namespace),
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
			Expect(resp.Names).Should(ContainElements(serviceName))
		})

		It("lists no services matching the prefix", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/servicesmatches/bogus",
				serverURL, v1.Root, namespace),
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

		It("lists all services matching the prefix", func() {
			By("querying matches")
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/servicesmatches/%s",
				serverURL, v1.Root, namespace, serviceName[:len(serviceName)-4]),
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
			Expect(resp.Names).Should(ContainElements(serviceName))
		})

		When("basic auth credentials are not provided", func() {
			It("returns a 401 response", func() {
				request, err := http.NewRequest("GET", fmt.Sprintf("%s%s/namespaces/%s/servicesmatches",
					serverURL, v1.Root, namespace), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				response, err := env.Client().Do(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
