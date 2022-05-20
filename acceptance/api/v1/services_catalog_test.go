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

var _ = Describe("ServiceCatalog Endpoint", func() {
	var catalogService models.CatalogService

	catalogResponse := func() models.ServiceCatalogResponse {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/catalogservices", serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var result models.ServiceCatalogResponse
		err = json.Unmarshal(bodyBytes, &result)
		Expect(err).ToNot(HaveOccurred(), string(bodyBytes))

		return result
	}

	BeforeEach(func() {
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
	})

	It("lists services from the 'epinio' namespace", func() {
		catalog.CreateCatalogService(catalogService)
		defer catalog.DeleteCatalogService(catalogService.Meta.Name)

		catalog := catalogResponse()
		serviceNames := []string{}
		for _, s := range catalog.CatalogServices {
			serviceNames = append(serviceNames, s.Meta.Name)
		}
		Expect(serviceNames).To(ContainElement(catalogService.Meta.Name))
	})

	It("doesn't list services from namespaces other than 'epinio'", func() {
		catalog.CreateCatalogServiceInNamespace("default", catalogService)
		defer catalog.DeleteCatalogServiceFromNamespace("default", catalogService.Meta.Name)

		catalog := catalogResponse()
		serviceNames := []string{}
		for _, s := range catalog.CatalogServices {
			serviceNames = append(serviceNames, s.Meta.Name)
		}
		Expect(serviceNames).ToNot(ContainElement(catalogService.Meta.Name))
	})
})
