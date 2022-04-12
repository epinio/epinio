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

var _ = Describe("ServiceShow Endpoint", func() {
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

	When("helmchart doesn't exist", func() {
		It("returns a 404", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/notexists", serverURL, v1.Root, namespace)
			response, err := env.Curl("GET", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	When("helmchart exists", func() {
		var serviceName string

		BeforeEach(func() {
			serviceName = catalog.NewServiceName()
		})

		When("helmchart is not labeled", func() {
			BeforeEach(func() {
				helmChart := helmapiv1.HelmChart{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "helm.cattle.io/v1",
						Kind:       "HelmChart",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      models.ServiceHelmChartName(serviceName, namespace),
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
				out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", models.ServiceHelmChartName(serviceName, namespace))
				Expect(err).ToNot(HaveOccurred(), out)
			})

			It("returns a 404", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceName)
				response, err := env.Curl("GET", endpoint, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		When("helmchart is labeled", func() {
			var helmChart helmapiv1.HelmChart

			BeforeEach(func() {
				helmChart = helmapiv1.HelmChart{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "helm.cattle.io/v1",
						Kind:       "HelmChart",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      models.ServiceHelmChartName(serviceName, namespace),
						Namespace: "epinio",
						Labels: map[string]string{
							services.CatalogServiceLabelKey:  catalogService.Name,
							services.TargetNamespaceLabelKey: namespace,
						},
					},
					Spec: helmapiv1.HelmChartSpec{
						TargetNamespace: namespace,
						Chart:           catalogService.HelmChart,
						Repo:            catalogService.HelmRepo.URL,
					},
				}
			})

			// Cleanup for all sub-cases
			AfterEach(func() {
				out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", models.ServiceHelmChartName(serviceName, namespace))
				Expect(err).ToNot(HaveOccurred(), out)
			})

			When("helmchart is ready", func() {

				BeforeEach(func() {
					createHelmChart(helmChart)
				})

				It("returns the service with status Ready", func() {
					Eventually(func() models.ServiceStatus {
						endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceName)
						response, err := env.Curl("GET", endpoint, strings.NewReader(""))
						Expect(err).ToNot(HaveOccurred())

						Expect(response.StatusCode).To(
							Equal(http.StatusOK),
							fmt.Sprintf("respose status was %d, not 200", response.StatusCode),
						)

						respBody, err := ioutil.ReadAll(response.Body)
						Expect(err).ToNot(HaveOccurred())

						var showResponse models.ServiceShowResponse
						err = json.Unmarshal(respBody, &showResponse)
						Expect(err).ToNot(HaveOccurred())
						Expect(showResponse.Service).ToNot(BeNil())

						return showResponse.Service.Status
					}, "1m", "5s").Should(Equal(models.ServiceStatusDeployed))
				})
			})

			When("helmchart is ready and the catalog service is missing", func() {

				BeforeEach(func() {
					helmChart.ObjectMeta.Labels[services.CatalogServiceLabelKey] = "missing-catalog-service"
					createHelmChart(helmChart)
				})

				It("returns the service with name prefixed with [Missing]", func() {
					Eventually(func() string {
						endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceName)
						response, err := env.Curl("GET", endpoint, strings.NewReader(""))
						Expect(err).ToNot(HaveOccurred())

						if response.StatusCode != http.StatusOK {
							return fmt.Sprintf("respose status was %d, not 200", response.StatusCode)
						}

						respBody, err := ioutil.ReadAll(response.Body)
						Expect(err).ToNot(HaveOccurred())

						var showResponse models.ServiceShowResponse
						err = json.Unmarshal(respBody, &showResponse)
						Expect(err).ToNot(HaveOccurred())
						Expect(showResponse.Service).ToNot(BeNil())

						return showResponse.Service.CatalogService
					}, "1m", "5s").Should(MatchRegexp("^\\[Missing\\].*"))
				})
			})

			When("helmchart is not ready", func() {
				BeforeEach(func() {
					helmChart.Spec.Chart = "doesntexist"
					createHelmChart(helmChart)
				})

				It("returns the service with status not-ready", func() {
					endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceName)
					response, err := env.Curl("GET", endpoint, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())

					Expect(response.StatusCode).To(Equal(http.StatusOK))

					var showResponse models.ServiceShowResponse
					err = json.NewDecoder(response.Body).Decode(&showResponse)
					Expect(err).ToNot(HaveOccurred())
					Expect(showResponse.Service).ToNot(BeNil())

					Expect(showResponse.Service.Status).To(Equal(models.ServiceStatusNotReady))
				})
			})
		})
	})
})
