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
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	helmapiv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
)

var _ = Describe("ServiceList Endpoint", func() {
	var namespace string
	var catalogService models.CatalogService

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		catalogService = models.CatalogService{
			Name:      catalog.NewCatalogServiceName(),
			HelmChart: "nginx",
			HelmRepo: models.HelmRepo{
				Name: "",
				URL:  "https://charts.bitnami.com/bitnami",
			},
			Values: "{'service': {'type': 'ClusterIP'}}",
		}
		createCatalogService(catalogService)
	})

	AfterEach(func() {
		deleteCatalogService(catalogService.Name)
		env.DeleteNamespace(namespace)
	})

	When("helmchart list is empty", func() {
		It("returns a 404", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace)
			response, err := env.Curl("GET", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())

			b, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			fmt.Printf("NS: %s - %+v\n", namespace, string(b))

			Expect(response.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	When("helmchart exists", func() {
		var serviceName1, serviceName2 string

		BeforeEach(func() {
			serviceName1 = catalog.NewServiceName()
			serviceName2 = catalog.NewServiceName()
		})

		When("helmchart is not labeled", func() {
			BeforeEach(func() {
				helmChart := helmapiv1.HelmChart{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "helm.cattle.io/v1",
						Kind:       "HelmChart",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      models.ServiceHelmChartName(serviceName1, namespace),
						Namespace: "epinio",
					},
					Spec: helmapiv1.HelmChartSpec{
						TargetNamespace: namespace,
						Chart:           catalogService.HelmChart,
						Repo:            catalogService.HelmRepo.URL,
					},
				}
				createHelmChart(helmChart)
			})

			AfterEach(func() {
				out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", models.ServiceHelmChartName(serviceName1, namespace))
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("returns a 404", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace)
				response, err := env.Curl("GET", endpoint, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		When("helmchart is labeled", func() {
			var helmChart1, helmChart2 helmapiv1.HelmChart

			BeforeEach(func() {
				commonHelmChart := helmapiv1.HelmChart{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "helm.cattle.io/v1",
						Kind:       "HelmChart",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "epinio",
						Labels: map[string]string{
							services.ServiceLabelKey: catalogService.Name,
						},
					},
					Spec: helmapiv1.HelmChartSpec{
						TargetNamespace: namespace,
						Chart:           catalogService.HelmChart,
						Repo:            catalogService.HelmRepo.URL,
					},
				}

				helmChart1 = commonHelmChart
				helmChart2 = commonHelmChart

				helmChart1.ObjectMeta.Name = models.ServiceHelmChartName(serviceName1, namespace)
				helmChart2.ObjectMeta.Name = models.ServiceHelmChartName(serviceName2, namespace)
			})

			// Cleanup for all sub-cases
			AfterEach(func() {
				out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", models.ServiceHelmChartName(serviceName1, namespace))
				Expect(err).ToNot(HaveOccurred(), out)

				out, err = proc.Kubectl("delete", "helmchart", "-n", "epinio", models.ServiceHelmChartName(serviceName2, namespace))
				Expect(err).ToNot(HaveOccurred(), out)
			})

			When("helmchart is ready", func() {

				BeforeEach(func() {
					createHelmChart(helmChart1)
					createHelmChart(helmChart2)
				})

				It("returns the two services", func() {
					endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace)
					response, err := env.Curl("GET", endpoint, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())

					Expect(response.StatusCode).To(
						Equal(http.StatusOK),
						fmt.Sprintf("respose status was %d, not 200", response.StatusCode),
					)

					respBody, err := ioutil.ReadAll(response.Body)
					Expect(err).ToNot(HaveOccurred())

					var listResponse models.ServiceListResponse
					err = json.Unmarshal(respBody, &listResponse)
					Expect(err).ToNot(HaveOccurred())

					Expect(listResponse.Services).ToNot(BeNil())
					Expect(listResponse.Services).To(HaveLen(2))

					Expect(listResponse.Services[0]).ToNot(BeEquivalentTo(listResponse.Services[1]))
				})
			})
		})
	})
})
