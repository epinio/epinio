package v1_test

import (
	"fmt"
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
			response, err := env.Curl("DELETE", endpoint, strings.NewReader(""))
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
			createCatalogService(catalogService)
		})

		AfterEach(func() {
			deleteCatalogService(catalogService.Name)
			env.DeleteNamespace(namespace)
		})

		When("service instance doesn't exist", func() {
			It("returns 404", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/notexists", serverURL, v1.Root, namespace)
				response, err := env.Curl("DELETE", endpoint, strings.NewReader(""))
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
							Chart:           catalogService.HelmChart,
							Repo:            catalogService.HelmRepo.URL,
						},
					}
					createHelmChart(helmChart)
				})

				AfterEach(func() {
					out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", names.ServiceHelmChartName(serviceName, namespace))
					Expect(err).ToNot(HaveOccurred(), out)
				})

				It("returns 404", func() {
					endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/notexists", serverURL, v1.Root, namespace)
					response, err := env.Curl("DELETE", endpoint, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())

					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			When("helmchart is labeled", func() {
				BeforeEach(func() {
					env.MakeServiceInstance(serviceName, catalogService.Name)
				})

				It("deletes the helmchart", func() {
					out, err := proc.Kubectl("get", "helmchart", "-n", "epinio", names.ServiceHelmChartName(serviceName, namespace))
					Expect(err).ToNot(HaveOccurred(), out)
					Expect(out).ToNot(MatchRegexp("helmcharts.helm.cattle.io.*not found"))

					endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s",
						serverURL, v1.Root, namespace, serviceName)
					response, err := env.Curl("DELETE", endpoint, strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusOK))

					Eventually(func() string {
						out, err = proc.Kubectl("get", "helmchart", "-n", "epinio", names.ServiceHelmChartName(serviceName, namespace))
						return out
					}, "1m", "5s").Should(MatchRegexp("helmcharts.helm.cattle.io.*not found"))
				})
			})
		})
	})
})
