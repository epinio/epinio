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

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const containerImageURL = "epinio/sample-app"

var _ = Describe("ServiceBatchBind Endpoint", LService, func() {
	var (
		namespace string
		appName   string
		catalog1  models.CatalogService
		catalog2  models.CatalogService
		service1  string
		service2  string
		service3  string
	)

	When("batch binding multiple services", func() {
		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			appName = catalog.NewAppName()
			env.MakeContainerImageApp(appName, 1, containerImageURL)

			catalog1 = catalog.NginxCatalogService(catalog.NewCatalogServiceName())
			catalog2 = catalog.RedisCatalogService(catalog.NewCatalogServiceName())

			service1 = catalog.NewServiceName()
			service2 = catalog.NewServiceName()
			service3 = catalog.NewServiceName()

			catalog.CreateService(service1, namespace, catalog1)
			catalog.CreateService(service2, namespace, catalog1)
			catalog.CreateService(service3, namespace, catalog2)
		})

		AfterEach(func() {
			env.DeleteApp(appName)
			catalog.DeleteService(service1, namespace)
			catalog.DeleteService(service2, namespace)
			catalog.DeleteService(service3, namespace)
			env.DeleteNamespace(namespace)
		})

		It("binds multiple services to an application", func() {
			request := models.ServiceBatchBindRequest{
				AppName:      appName,
				ServiceNames: []string{service1, service2, service3},
			}

			bodyBytes, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST",
				fmt.Sprintf("%s/api/v1/namespaces/%s/applications/%s/servicebindings",
					serverURL, namespace, appName),
				bytes.NewReader(bodyBytes))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err = io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			// Verify all services are bound
			appResponse := env.ShowApp(appName, namespace)
			Expect(appResponse.Configuration.Services).To(ConsistOf(service1, service2, service3))
		})

		It("returns error when application doesn't exist", func() {
			nonExistentApp := "nonexistent-app"
			request := models.ServiceBatchBindRequest{
				AppName:      nonExistentApp,
				ServiceNames: []string{service1},
			}

			bodyBytes, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST",
				fmt.Sprintf("%s/api/v1/namespaces/%s/applications/%s/servicebindings",
					serverURL, namespace, nonExistentApp),
				bytes.NewReader(bodyBytes))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("returns error when a service doesn't exist", func() {
			nonExistentService := "nonexistent-service"
			request := models.ServiceBatchBindRequest{
				AppName:      appName,
				ServiceNames: []string{service1, nonExistentService},
			}

			bodyBytes, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST",
				fmt.Sprintf("%s/api/v1/namespaces/%s/applications/%s/servicebindings",
					serverURL, namespace, appName),
				bytes.NewReader(bodyBytes))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("returns error when service list is empty", func() {
			request := models.ServiceBatchBindRequest{
				AppName:      appName,
				ServiceNames: []string{},
			}

			bodyBytes, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST",
				fmt.Sprintf("%s/api/v1/namespaces/%s/applications/%s/servicebindings",
					serverURL, namespace, appName),
				bytes.NewReader(bodyBytes))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})

		It("binds a single service using batch endpoint", func() {
			request := models.ServiceBatchBindRequest{
				AppName:      appName,
				ServiceNames: []string{service1},
			}

			bodyBytes, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST",
				fmt.Sprintf("%s/api/v1/namespaces/%s/applications/%s/servicebindings",
					serverURL, namespace, appName),
				bytes.NewReader(bodyBytes))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err = io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			// Verify service is bound
			appResponse := env.ShowApp(appName, namespace)
			Expect(appResponse.Configuration.Services).To(ContainElement(service1))
		})

		It("handles binding when some services are from different catalogs", func() {
			request := models.ServiceBatchBindRequest{
				AppName:      appName,
				ServiceNames: []string{service1, service3}, // nginx and redis
			}

			bodyBytes, err := json.Marshal(request)
			Expect(err).ToNot(HaveOccurred())

			response, err := env.Curl("POST",
				fmt.Sprintf("%s/api/v1/namespaces/%s/applications/%s/servicebindings",
					serverURL, namespace, appName),
				bytes.NewReader(bodyBytes))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err = io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			// Verify both services are bound
			appResponse := env.ShowApp(appName, namespace)
			Expect(appResponse.Configuration.Services).To(ConsistOf(service1, service3))
		})
	})
})
