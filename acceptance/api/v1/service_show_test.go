package v1_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceShow Endpoint", func() {
	var namespace string
	var catalogService models.CatalogService

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		catalogService = models.CatalogService{
			Meta: models.MetaLite{
				Name: catalog.NewCatalogServiceName(),
			},
			AppVersion: "1.2.3",
			HelmChart:  "nginx",
			HelmRepo: models.HelmRepo{
				Name: "",
				URL:  "https://charts.bitnami.com/bitnami",
			},
			Values: "{'service': {'type': 'ClusterIP'}}",
		}
		catalog.CreateCatalogService(catalogService)
	})

	AfterEach(func() {
		catalog.DeleteCatalogService(catalogService.Meta.Name)
		env.DeleteNamespace(namespace)
	})

	When("service doesn't exist", func() {
		It("returns a 404", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/notexists", serverURL, v1.Root, namespace)
			response, err := env.Curl("GET", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	When("service exists", func() {
		var serviceName string

		BeforeEach(func() {
			serviceName = catalog.NewServiceName()
		})

		When("service is not labeled", func() {
			BeforeEach(func() {
				catalog.CreateUnlabeledService(serviceName, namespace, catalogService)
			})

			AfterEach(func() {
				catalog.DeleteService(serviceName, namespace)
			})

			It("returns a 404", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceName)
				response, err := env.Curl("GET", endpoint, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		When("service is labeled", func() {
			// Cleanup for all sub-cases
			AfterEach(func() {
				catalog.DeleteService(serviceName, namespace)
			})

			When("service is ready", func() {
				BeforeEach(func() {
					catalog.CreateService(serviceName, namespace, catalogService)
				})

				It("returns the service with status Ready", func() {
					var showResponse models.Service

					Eventually(func() models.ServiceStatus {
						endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceName)
						response, err := env.Curl("GET", endpoint, strings.NewReader(""))
						Expect(err).ToNot(HaveOccurred())

						if response.StatusCode == http.StatusNotFound {
							return models.ServiceStatus(
								fmt.Sprintf("respose status was %d, not 200", response.StatusCode))
						}

						respBody, err := io.ReadAll(response.Body)
						Expect(err).ToNot(HaveOccurred())

						err = json.Unmarshal(respBody, &showResponse)
						Expect(err).ToNot(HaveOccurred())
						Expect(showResponse).ToNot(BeNil())

						return showResponse.Status
					}, "1m", "5s").Should(Equal(models.ServiceStatusDeployed))

					By("checking the catalog fields")
					Expect(showResponse.CatalogService).To(Equal(catalogService.Meta.Name))
					Expect(showResponse.CatalogServiceVersion).To(Equal(catalogService.AppVersion))
				})
			})

			When("service is ready and the catalog service is missing", func() {
				BeforeEach(func() {
					catalog.CreateServiceWithoutCatalog(serviceName, namespace, catalogService)
				})

				It("returns the service with name prefixed with [Missing]", func() {
					Eventually(func() string {
						endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceName)
						response, err := env.Curl("GET", endpoint, strings.NewReader(""))
						Expect(err).ToNot(HaveOccurred())

						if response.StatusCode != http.StatusOK {
							return fmt.Sprintf("respose status was %d, not 200", response.StatusCode)
						}

						respBody, err := io.ReadAll(response.Body)
						Expect(err).ToNot(HaveOccurred())

						var showResponse models.Service
						err = json.Unmarshal(respBody, &showResponse)
						Expect(err).ToNot(HaveOccurred())
						Expect(showResponse).ToNot(BeNil())

						return showResponse.CatalogService
					}, "1m", "5s").Should(MatchRegexp("^\\[Missing\\].*"))
				})
			})
		})
	})
})
