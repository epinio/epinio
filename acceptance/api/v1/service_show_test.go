package v1_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/names"
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
						Name:      names.ServiceHelmChartName(serviceName, namespace),
						Namespace: "epinio",
					},
					Spec: helmapiv1.HelmChartSpec{
						TargetNamespace: namespace,
						ValuesContent:   catalogService.Values,
						Chart:           catalogService.HelmChart,
						Repo:            catalogService.HelmRepo.URL,
					},
				}
				catalog.CreateHelmChart(helmChart, true)
			})

			AfterEach(func() {
				out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", names.ServiceHelmChartName(serviceName, namespace))
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
						Name:      names.ServiceHelmChartName(serviceName, namespace),
						Namespace: "epinio",
						Labels: map[string]string{
							services.CatalogServiceLabelKey:        catalogService.Meta.Name,
							services.TargetNamespaceLabelKey:       namespace,
							services.CatalogServiceVersionLabelKey: catalogService.AppVersion,
							services.ServiceNameLabelKey:           serviceName,
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
				out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", names.ServiceHelmChartName(serviceName, namespace))
				Expect(err).ToNot(HaveOccurred(), out)
			})

			When("helmchart is ready", func() {

				BeforeEach(func() {
					catalog.CreateHelmChart(helmChart, true)
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

			When("helmchart is ready and the catalog service is missing", func() {

				BeforeEach(func() {
					helmChart.ObjectMeta.Labels[services.CatalogServiceLabelKey] = "missing-catalog-service"
					catalog.CreateHelmChart(helmChart, true)
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

			When("helmchart is not ready", func() {
				BeforeEach(func() {
					helmChart.Spec.Chart = "doesntexist"
					catalog.CreateHelmChart(helmChart, false)
				})

				It("returns the service with status not-ready", func() {
					endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceName)
					response, err := env.Curl("GET", endpoint, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())

					Expect(response.StatusCode).To(Equal(http.StatusOK))

					var showResponse models.Service
					err = json.NewDecoder(response.Body).Decode(&showResponse)
					Expect(err).ToNot(HaveOccurred())
					Expect(showResponse).ToNot(BeNil())

					Expect(showResponse.Status).To(Equal(models.ServiceStatusNotReady))
				})
			})
		})
	})
})
