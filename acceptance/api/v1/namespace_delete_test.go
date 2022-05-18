package v1_test

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DELETE /api/v1/namespaces/:namespace", func() {
	const jsOK = `{"status":"ok"}`
	var namespace, otherNamespace, serviceName, otherService, containerImageURL string
	var catalogService models.CatalogService

	BeforeEach(func() {
		containerImageURL = "splatform/sample-app"

		// Create a Catalog Service
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

		// Irrelevant namespace and service instance
		otherNamespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(otherNamespace)
		otherService = catalog.NewServiceName()
		env.MakeServiceInstance(otherService, catalogService.Name)

		// The namespace under test
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		serviceName = catalog.NewServiceName()
		env.MakeServiceInstance(serviceName, catalogService.Name)

		// An app
		app1 := catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)

		// A Configuration
		conf1 := catalog.NewConfigurationName()
		env.MakeConfiguration(conf1)
		env.BindAppConfiguration(app1, conf1, namespace)
	})

	AfterEach(func() {
		out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", names.ServiceHelmChartName(otherService, otherNamespace))
		Expect(err).ToNot(HaveOccurred(), out)

		catalog.DeleteCatalogService(catalogService.Name)
		env.DeleteNamespace(otherNamespace)
	})

	It("deletes an namespace including apps, configurations and services", func() {
		response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/%s",
			serverURL, api.Root, namespace),
			strings.NewReader(``))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := ioutil.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(bodyBytes)).To(Equal(jsOK))

		env.VerifyNamespaceNotExist(namespace)
		out, err := proc.Kubectl("get", "helmchart", "-n", "epinio", names.ServiceHelmChartName(serviceName, namespace))
		Expect(err).To(HaveOccurred(), out)
		Expect(out).To(MatchRegexp("helmcharts.helm.cattle.io.*not found"))

		// Doesn't delete service from other namespace
		Consistently(func() string {
			out, err := proc.Kubectl("get", "helmchart", "-n", "epinio", names.ServiceHelmChartName(otherService, otherNamespace))
			Expect(err).ToNot(HaveOccurred(), out)
			return out
		}, "1m", "5s").Should(MatchRegexp(names.ServiceHelmChartName(otherService, otherNamespace))) // Expect not deleted

	})
})
