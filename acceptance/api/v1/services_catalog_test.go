package v1_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/helpers"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceCatalog Endpoint", func() {
	var serviceName string

	sampleServiceTmpFile := func(namespace, name string) string {
		serviceYAML := fmt.Sprintf(`
apiVersion: application.epinio.io/v1
kind: Service
metadata:
  name: "%[1]s"
  namespace: "%[2]s"
spec:
  chart: postgresql
  description: |
    A simple description of this service.
  helmRepo:
    name: bitnami
    url: https://charts.bitnami.com/bitnami
  name: %[1]s`, name, namespace)

		filePath, err := helpers.CreateTmpFile(serviceYAML)
		Expect(err).ToNot(HaveOccurred())

		return filePath
	}

	createService := func(namespace, name string) {
		sampleServiceFilePath := sampleServiceTmpFile(namespace, name)
		defer os.Remove(sampleServiceFilePath)

		out, err := proc.Kubectl("apply", "-f", sampleServiceFilePath)
		Expect(err).ToNot(HaveOccurred(), out)
	}

	deleteService := func(namespace, name string) {
		out, err := proc.Kubectl("delete", "-n", namespace, "services.application.epinio.io", name)
		Expect(err).ToNot(HaveOccurred(), out)
	}

	catalogResponse := func() models.ServiceCatalogResponse {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/services", serverURL, v1.Root), strings.NewReader(""))
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
		serviceName = catalog.NewCatalogServiceName()
	})

	It("lists services from the 'epinio' namespace", func() {
		createService("epinio", serviceName)
		defer deleteService("epinio", serviceName)

		catalog := catalogResponse()
		serviceNames := []string{}
		for _, s := range catalog.Services {
			serviceNames = append(serviceNames, s.Name)
		}
		Expect(serviceNames).To(ContainElement(serviceName))
	})

	It("doesn't list services from namespaces other than 'epinio'", func() {
		createService("default", serviceName)
		defer deleteService("default", serviceName)

		catalog := catalogResponse()
		serviceNames := []string{}
		for _, s := range catalog.Services {
			serviceNames = append(serviceNames, s.Name)
		}
		Expect(serviceNames).ToNot(ContainElement(serviceName))
	})
})
