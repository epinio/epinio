package v1_test

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	apiv1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceUnbind Endpoint", func() {
	var namespace, containerImageURL, app, serviceName string
	var catalogService models.CatalogService

	BeforeEach(func() {
		containerImageURL = "splatform/sample-app"

		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		catalogService = models.CatalogService{
			Name:      catalog.NewCatalogServiceName(),
			HelmChart: "mysql",
			HelmRepo: models.HelmRepo{
				Name: "",
				URL:  "https://charts.bitnami.com/bitnami",
			},
		}

		catalog.CreateCatalogService(catalogService)

		app = catalog.NewAppName()
		env.MakeContainerImageApp(app, 1, containerImageURL)

		serviceName = catalog.NewServiceName()
		catalog.CreateService(serviceName, namespace, catalogService)

		// Bind the service to the app
		out, err := env.Epinio("", "service", "bind", serviceName, app)
		Expect(err).ToNot(HaveOccurred(), out)
		out, err = env.Epinio("", "app", "show", app)
		Expect(err).ToNot(HaveOccurred(), out)
		matchString := fmt.Sprintf("Bound Configurations.*%s", serviceName)
		Expect(out).To(MatchRegexp(matchString))
	})

	AfterEach(func() {
		env.DeleteApp(app)
		out, err := proc.Kubectl("delete", "helmchart", "-n", "epinio", names.ServiceHelmChartName(serviceName, namespace))
		Expect(err).ToNot(HaveOccurred(), out)

		catalog.DeleteCatalogService(catalogService.Name)
		env.DeleteNamespace(namespace)
	})

	It("Unbinds the service", func() {
		endpoint := fmt.Sprintf("%s%s/%s",
			serverURL, apiv1.Root, apiv1.Routes.Path("ServiceUnbind", namespace, serviceName))
		requestBody, err := json.Marshal(models.ServiceBindRequest{AppName: app})
		Expect(err).ToNot(HaveOccurred())
		response, err := env.Curl("POST", endpoint, strings.NewReader(string(requestBody)))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		appShowOut, err := env.Epinio("", "app", "show", app)
		Expect(err).ToNot(HaveOccurred())
		matchString := fmt.Sprintf("Bound Configurations.*%s", serviceName)
		Expect(appShowOut).ToNot(MatchRegexp(matchString))
	})
})
