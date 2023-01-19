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
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceList Endpoint", LService, func() {
	var namespace1, namespace2 string
	var catalogService models.CatalogService

	BeforeEach(func() {
		namespace1 = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace1)

		namespace2 = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace2)

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
		env.DeleteNamespace(namespace1)
		env.DeleteNamespace(namespace2)
	})

	When("no service exists", func() {
		It("returns a 200 with an empty list", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace1)
			response, err := env.Curl("GET", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusOK))

			var serviceListResponse models.ServiceList
			err = json.NewDecoder(response.Body).Decode(&serviceListResponse)
			Expect(err).ToNot(HaveOccurred())

			Expect(serviceListResponse).Should(HaveLen(0))
		})
	})

	When("only one service exists", func() {
		var serviceName1 string

		BeforeEach(func() {
			serviceName1 = catalog.NewServiceName()
		})

		When("it is in another namespace", func() {
			BeforeEach(func() {
				env.TargetNamespace(namespace2)
				env.MakeServiceInstance(serviceName1, catalogService.Meta.Name)
			})

			AfterEach(func() {
				catalog.DeleteService(serviceName1, namespace2)
			})

			It("returns an empty list", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace1)
				response, err := env.Curl("GET", endpoint, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusOK))

				var serviceListResponse models.ServiceList
				err = json.NewDecoder(response.Body).Decode(&serviceListResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(serviceListResponse).Should(HaveLen(0))
			})
		})

		When("it is in the targeted namespace", func() {
			BeforeEach(func() {
				env.TargetNamespace(namespace1)
				env.MakeServiceInstance(serviceName1, catalogService.Meta.Name)
			})

			AfterEach(func() {
				catalog.DeleteService(serviceName1, namespace1)
			})

			It("returns the list with the service", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace1)
				response, err := env.Curl("GET", endpoint, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusOK))

				var serviceListResponse models.ServiceList
				err = json.NewDecoder(response.Body).Decode(&serviceListResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(serviceListResponse).Should(HaveLen(1))
				Expect(serviceListResponse[0].Meta.Name).To(Equal(serviceName1))
			})
		})
	})

	When("two services exists", func() {
		var serviceName1, serviceName2 string

		BeforeEach(func() {
			serviceName1 = catalog.NewServiceName()
			serviceName2 = catalog.NewServiceName()
		})

		When("they are in another namespace", func() {
			BeforeEach(func() {
				env.TargetNamespace(namespace2)

				env.MakeServiceInstance(serviceName1, catalogService.Meta.Name)
				env.MakeServiceInstance(serviceName2, catalogService.Meta.Name)
			})

			AfterEach(func() {
				catalog.DeleteService(serviceName1, namespace2)
				catalog.DeleteService(serviceName2, namespace2)
			})

			It("returns an empty list", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace1)
				response, err := env.Curl("GET", endpoint, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusOK))

				var serviceListResponse models.ServiceList
				err = json.NewDecoder(response.Body).Decode(&serviceListResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(serviceListResponse).Should(HaveLen(0))
			})
		})

		When("they are in two different namespace", func() {
			BeforeEach(func() {
				env.TargetNamespace(namespace1)
				env.MakeServiceInstance(serviceName1, catalogService.Meta.Name)

				env.TargetNamespace(namespace2)
				env.MakeServiceInstance(serviceName2, catalogService.Meta.Name)
			})

			AfterEach(func() {
				catalog.DeleteService(serviceName1, namespace1)
				catalog.DeleteService(serviceName2, namespace2)
			})

			It("returns a list with service1 in namespace1", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace1)
				response, err := env.Curl("GET", endpoint, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusOK))

				var serviceListResponse models.ServiceList
				err = json.NewDecoder(response.Body).Decode(&serviceListResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(serviceListResponse).Should(HaveLen(1))
				Expect(serviceListResponse[0].Meta.Name).To(Equal(serviceName1))
			})

			It("returns a list with service2 in namespace2", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace2)
				response, err := env.Curl("GET", endpoint, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusOK))

				var serviceListResponse models.ServiceList
				err = json.NewDecoder(response.Body).Decode(&serviceListResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(serviceListResponse).Should(HaveLen(1))
				Expect(serviceListResponse[0].Meta.Name).To(Equal(serviceName2))
			})
		})

		When("they are in the targeted namespace", func() {
			BeforeEach(func() {
				env.TargetNamespace(namespace1)

				env.MakeServiceInstance(serviceName1, catalogService.Meta.Name)
				env.MakeServiceInstance(serviceName2, catalogService.Meta.Name)
			})

			AfterEach(func() {
				catalog.DeleteService(serviceName1, namespace1)
				catalog.DeleteService(serviceName2, namespace1)
			})

			It("returns a list with both services", func() {
				endpoint := fmt.Sprintf("%s%s/namespaces/%s/services", serverURL, v1.Root, namespace1)
				response, err := env.Curl("GET", endpoint, strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())

				Expect(response.StatusCode).To(Equal(http.StatusOK))

				var serviceListResponse models.ServiceList
				err = json.NewDecoder(response.Body).Decode(&serviceListResponse)
				Expect(err).ToNot(HaveOccurred())

				Expect(serviceListResponse).Should(HaveLen(2))
			})
		})
	})
})
