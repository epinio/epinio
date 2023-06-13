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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceCreate Endpoint", LService, func() {
	var namespace string

	When("namespace doesn't exist", func() {
		It("returns an error", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/doesntexist/services", serverURL, v1.Root)
			response, err := env.Curl("POST", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound))
		})
	})

	When("body is empty", func() {
		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)
		})

		AfterEach(func() {
			env.DeleteNamespace(namespace)
		})

		It("returns an error", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace)
			response, err := env.Curl("POST", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	When("service does not exist", func() {
		var requestBody, serviceName string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)
			serviceName = catalog.NewServiceName()

			service := models.ServiceCreateRequest{
				CatalogService: "not-existing",
				Name:           serviceName,
			}

			b, err := json.Marshal(service)
			Expect(err).ToNot(HaveOccurred())
			requestBody = string(b)
		})

		AfterEach(func() {
			env.DeleteNamespace(namespace)
		})

		It("returns an error", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace)
			response, err := env.Curl("POST", endpoint, strings.NewReader(requestBody))
			Expect(err).ToNot(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	When("service exists", func() {
		var catalogService models.CatalogService

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			catalogService = models.CatalogService{
				Meta: models.MetaLite{
					Name: catalog.NewCatalogServiceName(),
				},
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
			catalog.DeleteCatalogService(catalogService.Meta.Name)
			env.DeleteNamespace(namespace)
		})

		It("returns success immediately if waiting", func() {
			serviceCreateRequest := models.ServiceCreateRequest{
				CatalogService: catalogService.Meta.Name,
				Name:           catalog.NewServiceName(),
				Wait:           true,
			}

			requestBody := &bytes.Buffer{}
			err := json.NewEncoder(requestBody).Encode(serviceCreateRequest)
			Expect(err).ToNot(HaveOccurred())

			endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace)
			response, err := env.Curl("POST", endpoint, requestBody)
			Expect(err).ToNot(HaveOccurred())

			b, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(b))

			endpoint = fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceCreateRequest.Name)
			response, err = env.Curl("GET", endpoint, nil)
			Expect(err).ToNot(HaveOccurred())

			var service models.Service
			err = json.NewDecoder(response.Body).Decode(&service)
			Expect(err).ToNot(HaveOccurred())
			Expect(service.Status.String()).To(BeEquivalentTo(models.ServiceStatusDeployed))

			catalog.DeleteService(serviceCreateRequest.Name, namespace)
		})

		It("returns success after the deploy is ok if not waiting", func() {
			serviceCreateRequest := models.ServiceCreateRequest{
				CatalogService: catalogService.Meta.Name,
				Name:           catalog.NewServiceName(),
				Wait:           false,
			}

			requestBody := &bytes.Buffer{}
			err := json.NewEncoder(requestBody).Encode(serviceCreateRequest)
			Expect(err).ToNot(HaveOccurred())

			endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace)
			response, err := env.Curl("POST", endpoint, requestBody)
			Expect(err).ToNot(HaveOccurred())

			b, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			response.Body.Close()

			Expect(response.StatusCode).To(Equal(http.StatusOK), string(b))

			Eventually(func() string {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services/%s", serverURL, v1.Root, namespace, serviceCreateRequest.Name)
				response, err := env.Curl("GET", endpoint, nil)
				Expect(err).ToNot(HaveOccurred())

				var service models.Service
				err = json.NewDecoder(response.Body).Decode(&service)
				Expect(err).ToNot(HaveOccurred())

				return service.Status.String()
			}, "1m", "1s").Should(BeEquivalentTo(models.ServiceStatusDeployed))

			catalog.DeleteService(serviceCreateRequest.Name, namespace)
		})
	})
})
