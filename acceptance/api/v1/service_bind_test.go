package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	helmapiv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ServiceBind Endpoint", func() {
	var namespace, containerImageURL string
	var catalogService models.CatalogService

	createService := func(name, namespace string, catalogService models.CatalogService) {
		helmChart := helmapiv1.HelmChart{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "helm.cattle.io/v1",
				Kind:       "HelmChart",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      models.ServiceHelmChartName(name, namespace),
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
		createHelmChart(helmChart)

		cmd := func() (string, error) {
			return proc.Run("", false, "helm", "get", "all", "-n", namespace,
				models.ServiceHelmChartName(name, namespace))
		}
		Eventually(func() error {
			_, err := cmd()
			return err
		}, "1m", "5s").Should(BeNil())

		Eventually(func() string {
			out, _ := cmd()
			return out
		}, "1m", "5s").ShouldNot(MatchRegexp(".*release: not found.*"))
	}

	BeforeEach(func() {
		containerImageURL = "splatform/sample-app"

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
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	When("the service doesn't exist", func() {
		var app string

		BeforeEach(func() {
			// Let's create an app so that only the service is missing
			app = catalog.NewAppName()
			env.MakeContainerImageApp(app, 1, containerImageURL)
		})

		AfterEach(func() {
			env.DeleteApp(app)
		})

		It("returns 404", func() {
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBind", namespace, "bogus"))
			requestBody, err := json.Marshal(models.ServiceBindRequest{AppName: app})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	When("the application doesn't exist", func() {
		var serviceName string

		BeforeEach(func() {
			createCatalogService(catalogService)

			// Let's create a service so that only app is missing
			serviceName = catalog.NewServiceName()
			createService(serviceName, namespace, catalogService)
		})

		AfterEach(func() {
			out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", models.ServiceHelmChartName(serviceName, namespace))
			Expect(err).ToNot(HaveOccurred(), out)

			deleteCatalogService(catalogService.Name)
		})

		It("returns 404", func() {
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBind", namespace, serviceName))
			requestBody, err := json.Marshal(models.ServiceBindRequest{AppName: "bogus"})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	When("both app and service exist", func() {
		var app, serviceName string

		BeforeEach(func() {
			// Use a chart that creates some secret (nginx doesn't)
			catalogService.HelmChart = "mysql"
			catalogService.Values = ""
			createCatalogService(catalogService)

			app = catalog.NewAppName()
			serviceName = catalog.NewServiceName()

			env.MakeContainerImageApp(app, 1, containerImageURL)
			createService(serviceName, namespace, catalogService)
		})

		AfterEach(func() {
			env.DeleteApp(app)
			out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", models.ServiceHelmChartName(serviceName, namespace))
			Expect(err).ToNot(HaveOccurred(), out)

			deleteCatalogService(catalogService.Name)
		})

		It("binds the service's secrets", func() {
			endpoint := fmt.Sprintf("%s%s/%s",
				serverURL, apiv1.Root, apiv1.Routes.Path("ServiceBind", namespace, serviceName))
			requestBody, err := json.Marshal(models.ServiceBindRequest{AppName: app})
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			appShowOut, err := env.Epinio("", "app", "show", app)
			Expect(err).ToNot(HaveOccurred())
			matchString := fmt.Sprintf("Bound Configurations.*%s", serviceName)
			Expect(appShowOut).To(MatchRegexp(matchString))
		})
	})
})
