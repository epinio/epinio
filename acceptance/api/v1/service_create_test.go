package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceCreate Endpoint", func() {
	var namespace string

	When("namespace doesn't exist", func() {
		It("returns an error", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/doesntexist/services", serverURL, v1.Root)
			response, err := env.Curl("POST", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	When("body is empty", func() {
		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)
		})

		AfterEach(func() {
			env.DeleteNamespace(namespace)
		})

		It("returns an error", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace)
			response, err := env.Curl("POST", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	When("service does not exist", func() {
		var requestBody, serviceName string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)
			serviceName = catalog.NewServiceName()

			service := models.ServiceCreateRequest{
				CatalogService: "not-existing",
				Name:           serviceName,
			}

			b, err := json.Marshal(service)
			Expect(err).ToNot(HaveOccurred())
			requestBody = string(b)
		})

		AfterEach(func() {
			env.DeleteNamespace(namespace)
		})

		It("returns an error", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace)
			response, err := env.Curl("POST", endpoint, strings.NewReader(requestBody))
			Expect(err).ToNot(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	When("service exists", func() {
		var requestBody, serviceName string
		var catalogService models.CatalogService

		deleteService := func(name, namespace string) {
			out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio",
				names.ServiceHelmChartName(name, namespace))
			Expect(err).ToNot(HaveOccurred(), out)
		}

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			catalogService = models.CatalogService{
				Meta: models.MetaLite{
					Name: catalog.NewCatalogServiceName(),
				},
				HelmChart: "nginx",
				HelmRepo: models.HelmRepo{
					Name: "",
					URL:  "https://charts.bitnami.com/bitnami",
				},
				Values: "{'service': {'type': 'ClusterIP'}}",
			}
			catalog.CreateCatalogService(catalogService)

			serviceName = catalog.NewServiceName()
			service := models.ServiceCreateRequest{
				CatalogService: catalogService.Meta.Name,
				Name:           serviceName,
			}

			b, err := json.Marshal(service)
			Expect(err).ToNot(HaveOccurred())
			requestBody = string(b)
		})

		AfterEach(func() {
			catalog.DeleteCatalogService(catalogService.Meta.Name)
			env.DeleteNamespace(namespace)
		})

		It("returns success", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace)
			response, err := env.Curl("POST", endpoint, strings.NewReader(requestBody))
			Expect(err).ToNot(HaveOccurred())
			defer deleteService(serviceName, namespace)

			b, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			response.Body.Close()

			Expect(response.StatusCode).To(Equal(http.StatusOK), string(b))
		})
	})
})
