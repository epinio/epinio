// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceDelete Endpoint", LService, func() {
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

			catalogService = catalog.CreateCatalogServiceNginx()
		})

		AfterEach(func() {
			catalog.DeleteCatalogService(catalogService.Meta.Name)
			env.DeleteNamespace(namespace)
		})

		When("service instance doesn't exist", func() {
			It("returns 404", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/notexists",
					serverURL, v1.Root, namespace)

				requestBody, err := json.Marshal(models.ServiceDeleteRequest{})
				Expect(err).ToNot(HaveOccurred())

				response, err := env.Curl("DELETE", endpoint,
					strings.NewReader(string(requestBody)))
				Expect(err).ToNot(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		When("service exists", func() {
			var serviceName string
			var releaseName string
			var secName string

			BeforeEach(func() {
				serviceName = catalog.NewServiceName()
				releaseName = names.ServiceReleaseName(serviceName)
				secName = names.GenerateResourceName("s", serviceName)
			})

			When("service is not labeled", func() {
				BeforeEach(func() {
					catalog.CreateUnlabeledService(serviceName, namespace, catalogService)
				})

				AfterEach(func() {
					catalog.DeleteService(serviceName, namespace)
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

			When("service is labeled", func() {
				BeforeEach(func() {
					catalog.CreateService(serviceName, namespace, catalogService)

					By("locate service secret: " + secName + ", for: " + serviceName)
					Eventually(func() string {
						out, err := proc.Kubectl("get", "secret", "--namespace", namespace, secName)
						Expect(err).ToNot(HaveOccurred(), out)
						return out
					}, "3m", "5s").ShouldNot(MatchRegexp("No resources found"))

					By("locate service helm release: " + releaseName + ", for: " + serviceName)
					Eventually(func() string {
						out, err := proc.Kubectl("get", "secret", "--namespace", namespace,
							"--selector", "name="+releaseName)
						Expect(err).ToNot(HaveOccurred(), out)
						return out
					}, "3m", "5s").ShouldNot(MatchRegexp("No resources found"))
				})

				It("deletes the helm release", func() {
					By("assemble url")
					endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceName)

					By(fmt.Sprintf("assemble request for %s", endpoint))
					requestBody, err := json.Marshal(models.ServiceDeleteRequest{})
					Expect(err).ToNot(HaveOccurred())

					By("curl request")
					response, err := env.Curl("DELETE", endpoint, strings.NewReader(string(requestBody)))
					Expect(err).ToNot(HaveOccurred())

					By("read response")
					respBody, err := io.ReadAll(response.Body)
					Expect(err).ToNot(HaveOccurred())

					By(fmt.Sprintf("decode response %s", string(respBody)))
					var deleteResponse models.ServiceDeleteResponse
					err = json.Unmarshal(respBody, &deleteResponse)
					Expect(err).ToNot(HaveOccurred(), string(respBody))

					By("check status")
					Expect(response.StatusCode).To(Equal(http.StatusOK), string(respBody))

					By("check helm release removal: " + releaseName)
					Eventually(func() string {
						out, err := proc.Kubectl("get", "secret", "--namespace", namespace, "--selector", "name="+releaseName)
						Expect(err).ToNot(HaveOccurred(), out)
						return out
					}, "1m", "5s").Should(MatchRegexp("No resources found"))
				})
			})

			Context("multi service deletion", func() {
				var svc1, svc2 string
				var requestBody string

				BeforeEach(func() {
					svc1 = catalog.NewServiceName()
					svc2 = catalog.NewServiceName()

					env.MakeServiceInstance(svc1, catalogService.Meta.Name)
					env.MakeServiceInstance(svc2, catalogService.Meta.Name)

					requestBody = `{ "unbind": false }`
				})

				It("deletes multiple services", func() {
					makeServiceDeleteRequest(namespace, requestBody, svc1, svc2)
					verifyServicesDeleted(namespace, svc1, svc2)
				})
			})
		})
	})
})

func makeServiceDeleteRequest(namespace, requestBody string, serviceNames ...string) {
	By(fmt.Sprintf("Deleting services  %+v", serviceNames))

	q := url.Values{}
	for _, c := range serviceNames {
		q.Add("services[]", c)
	}
	URLParams := q.Encode()

	response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/%s/services?%s",
		serverURL, v1.Root, namespace, URLParams), strings.NewReader(requestBody))
	Expect(err).ToNot(HaveOccurred())
	Expect(response).ToNot(BeNil())

	defer response.Body.Close()
	bodyBytes, err := io.ReadAll(response.Body)
	Expect(err).ToNot(HaveOccurred())
	Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

	By(fmt.Sprintf("__Response: %s", string(bodyBytes)))

	var responseData models.Response
	err = json.Unmarshal(bodyBytes, &responseData)
	Expect(err).ToNot(HaveOccurred())
}

func verifyServicesDeleted(namespace string, serviceNames ...string) {
	By(fmt.Sprintf("Verifying deletion %+v", serviceNames))

	Eventually(func() []string {
		// 1. List services
		responseGet, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/services",
			serverURL, v1.Root, namespace), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(responseGet).ToNot(BeNil())
		defer responseGet.Body.Close()
		bodyBytesGet, err := io.ReadAll(responseGet.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(responseGet.StatusCode).To(Equal(http.StatusOK))

		var data models.ServiceList
		err = json.Unmarshal(bodyBytesGet, &data)
		Expect(err).ToNot(HaveOccurred())

		var existingServices []string
		for _, conf := range data {
			existingServices = append(existingServices, conf.Meta.Name)
		}

		return existingServices
		// 2. Check that listing does not contain the removed ones
	}, "2m").ShouldNot(ContainElements(serviceNames))
}
