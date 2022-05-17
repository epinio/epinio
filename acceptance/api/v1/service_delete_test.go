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
	helmapiv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceDelete Endpoint", func() {
	var namespace string
	var catalogService models.CatalogService

	When("namespace doesn't exist", func() {
		It("returns 404", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/notexists/services/whatever", serverURL, v1.Root)

			requestBody, err := json.Marshal(models.ServiceDeleteRequest{})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("DELETE", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	When("namespace exists", func() {
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
			catalog.CreateCatalogService(catalogService)
		})

		AfterEach(func() {
			catalog.DeleteCatalogService(catalogService.Name)
			env.DeleteNamespace(namespace)
		})

		When("service instance doesn't exist", func() {
			It("returns 404", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/notexists", serverURL, v1.Root, namespace)

				requestBody, err := json.Marshal(models.ServiceDeleteRequest{})
				Expect(err).ToNot(HaveOccurred())

				response, err := env.Curl("DELETE", endpoint, strings.NewReader(string(requestBody)))
				Expect(err).ToNot(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		When("helmchart exists", func() {
			var serviceName string
			var chartName string

			BeforeEach(func() {
				serviceName = catalog.NewServiceName()
				chartName = names.ServiceHelmChartName(serviceName, namespace)
			})

			When("helmchart is not labeled", func() {
				BeforeEach(func() {
					helmChart := helmapiv1.HelmChart{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "helm.cattle.io/v1",
							Kind:       "HelmChart",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      chartName,
							Namespace: "epinio",
						},
						Spec: helmapiv1.HelmChartSpec{
							TargetNamespace: namespace,
							Chart:           catalogService.HelmChart,
							Repo:            catalogService.HelmRepo.URL,
						},
					}
					catalog.CreateHelmChart(helmChart)
				})

				AfterEach(func() {
					out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", chartName)
					Expect(err).ToNot(HaveOccurred(), out)
				})

				It("returns 404", func() {
					endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceName)

					requestBody, err := json.Marshal(models.ServiceDeleteRequest{})
					Expect(err).ToNot(HaveOccurred())

					response, err := env.Curl("DELETE", endpoint, strings.NewReader(string(requestBody)))
					Expect(err).ToNot(HaveOccurred())

					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			When("helmchart is labeled", func() {
				BeforeEach(func() {
					env.MakeServiceInstance(serviceName, catalogService.Name)

					By(fmt.Sprintf("locate helm chart %s", chartName))

					out, err := proc.Kubectl("get", "helmchart", "-n", "epinio", chartName)
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(MatchRegexp("helmcharts.helm.cattle.io.*not found"))
				})

				It("deletes the helmchart", func() {
					By("assemble url")
					endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s",
						serverURL, v1.Root, namespace, serviceName)

					By(fmt.Sprintf("assemble request for %s", endpoint))
					requestBody, err := json.Marshal(models.ServiceDeleteRequest{})
					Expect(err).ToNot(HaveOccurred())

					By("curl request")
					response, err := env.Curl("DELETE", endpoint, strings.NewReader(string(requestBody)))
					Expect(err).ToNot(HaveOccurred())

					By("read response")
					respBody, err := ioutil.ReadAll(response.Body)
					Expect(err).ToNot(HaveOccurred())

					By(fmt.Sprintf("decode response %s", string(respBody)))
					var deleteResponse models.ServiceDeleteResponse
					err = json.Unmarshal(respBody, &deleteResponse)
					Expect(err).ToNot(HaveOccurred(), string(respBody))

					By("check status")
					Expect(response.StatusCode).To(Equal(http.StatusOK), string(respBody))

					By("check helm chart removal")
					Eventually(func() string {
						out, _ := proc.Kubectl("get", "helmchart", "-n", "epinio", chartName)
						return out
					}, "1m", "5s").Should(MatchRegexp("helmcharts.helm.cattle.io.*not found"))
				})
			})
		})
	})
})
