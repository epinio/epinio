package v1_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configurations API Application Endpoints", func() {
	Describe("DELETE /api/v1/namespaces/:namespace/configurations", func() {
		var namespace string
		var svc1, svc2 string
		var requestBody string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			svc1 = catalog.NewConfigurationName()
			svc2 = catalog.NewConfigurationName()

			env.MakeConfiguration(svc1)
			env.MakeConfiguration(svc2)

			requestBody = `{ "unbind": false }`
		})

		When("namespace doesn't exist", func() {
			It("returns 404", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/notexists/configurations/whatever",
					serverURL, api.Root)
				response, err := env.Curl("DELETE", endpoint,
					strings.NewReader(string(requestBody)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		It("deletes multiple configurations", func() {
			makeConfigurationDeleteRequest(namespace, requestBody, svc1, svc2)
			verifyConfigurationsDeleted(namespace, svc1, svc2)
		})

		When("the configurations are bound to applications", func() {
			var containerImageURL string
			var appName string

			BeforeEach(func() {
				requestBody = `{ "unbind": true }`

				containerImageURL = "splatform/sample-app"
				appName = catalog.NewAppName()

				env.MakeContainerImageApp(appName, 1, containerImageURL)
				env.BindAppConfiguration(appName, svc1, namespace)
			})

			It("deletes and unbinds them", func() {
				makeConfigurationDeleteRequest(namespace, requestBody, svc1, svc2)
				verifyConfigurationsDeleted(namespace, svc1, svc2)
			})
		})
	})
})

func makeConfigurationDeleteRequest(namespace, requestBody string, configurationNames ...string) {
	q := url.Values{}
	for _, c := range configurationNames {
		q.Add("configurations[]", c)
	}
	URLParams := q.Encode()

	response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/%s/configurations?%s",
		serverURL, api.Root, namespace, URLParams), strings.NewReader(requestBody))
	Expect(err).ToNot(HaveOccurred())
	Expect(response).ToNot(BeNil())

	defer response.Body.Close()
	bodyBytes, err := io.ReadAll(response.Body)
	Expect(err).ToNot(HaveOccurred())
	Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

	var responseData models.Response
	err = json.Unmarshal(bodyBytes, &responseData)
	Expect(err).ToNot(HaveOccurred())
}

func verifyConfigurationsDeleted(namespace string, configurationNames ...string) {
	// Confirm that they are now deleted
	responseGet, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/configurations",
		serverURL, api.Root, namespace), strings.NewReader(""))
	Expect(err).ToNot(HaveOccurred())
	Expect(responseGet).ToNot(BeNil())
	defer responseGet.Body.Close()
	bodyBytesGet, err := io.ReadAll(responseGet.Body)
	Expect(err).ToNot(HaveOccurred())
	Expect(responseGet.StatusCode).To(Equal(http.StatusOK))

	var data models.ConfigurationResponseList
	err = json.Unmarshal(bodyBytesGet, &data)
	Expect(err).ToNot(HaveOccurred())

	var existingConfigurations []string
	for _, conf := range data {
		existingConfigurations = append(existingConfigurations, conf.Meta.Name)
	}

	Expect(existingConfigurations).ToNot(ContainElements(configurationNames))
}
