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
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceList Endpoint", LService, func() {
	var namespace1, namespace2 string
	var catalogService, otherCatalogService models.CatalogService

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

		otherCatalogService = models.CatalogService{
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
		catalog.CreateCatalogService(otherCatalogService)
	})

	AfterEach(func() {
		catalog.DeleteCatalogService(catalogService.Meta.Name)
		catalog.DeleteCatalogService(otherCatalogService.Meta.Name)
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

	Describe("GET /api/v1/services search", func() {
		var serviceName1, serviceName2 string

		BeforeEach(func() {
			serviceName1 = catalog.NewServiceName()
			serviceName2 = catalog.NewServiceName()

			env.TargetNamespace(namespace1)
			env.MakeServiceInstance(serviceName1, catalogService.Meta.Name)

			env.TargetNamespace(namespace2)
			env.MakeServiceInstance(serviceName2, catalogService.Meta.Name)
		})

		AfterEach(func() {
			catalog.DeleteService(serviceName1, namespace1)
			catalog.DeleteService(serviceName2, namespace2)
		})

		It("filters services by search term", func() {
			endpoint := fmt.Sprintf("%s%s/services?search=%s", serverURL, v1.Root, serviceName1)
			response, err := env.Curl("GET", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			var serviceListResponse models.ServiceList
			err = json.NewDecoder(response.Body).Decode(&serviceListResponse)
			Expect(err).ToNot(HaveOccurred())

			Expect(serviceListResponse).ToNot(BeEmpty())
			found := false
			for _, svc := range serviceListResponse {
				Expect(svc.Meta.Name).To(ContainSubstring(serviceName1))
				if svc.Meta.Name == serviceName1 {
					found = true
				}
			}
			Expect(found).To(BeTrue(), "expected search results to contain %q", serviceName1)
		})
	})

	Describe("GET /api/v1/services namespaces", func() {
		var serviceName1, serviceName2 string
		var user, password string

		BeforeEach(func() {
			serviceName1 = catalog.NewServiceName()
			serviceName2 = catalog.NewServiceName()

			env.TargetNamespace(namespace1)
			env.MakeServiceInstance(serviceName1, catalogService.Meta.Name)

			env.TargetNamespace(namespace2)
			env.MakeServiceInstance(serviceName2, catalogService.Meta.Name)

			user, password = env.CreateEpinioUser("user", nil)
		})

		AfterEach(func() {
			catalog.DeleteService(serviceName1, namespace1)
			catalog.DeleteService(serviceName2, namespace2)

			env.DeleteEpinioUser(user)
		})

		listServices := func(query string) models.ServiceList {
			endpoint := fmt.Sprintf("%s%s/services?%s",
				serverURL, v1.Root, query)

			response, err := env.Curl("GET", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			defer response.Body.Close()

			var services models.ServiceList
			err = json.NewDecoder(response.Body).Decode(&services)
			Expect(err).ToNot(HaveOccurred())

			return services
		}

		serviceRefsOf := func(services models.ServiceList) [][]string {
			var serviceRefs [][]string
			for _, svc := range services {
				serviceRefs = append(serviceRefs, []string{
					svc.Meta.Name,
					svc.Meta.Namespace,
				})
			}

			return serviceRefs
		}

		// Every filter case shares one fixture: provisioning two service
		// instances is far more expensive than the reads themselves.
		It("filters services by namespace", func() {
			By("keeping only the requested namespace")
			services := listServices("namespaces=" + namespace1)
			Expect(serviceRefsOf(services)).To(ConsistOf(
				[]string{serviceName1, namespace1}))

			By("keeping the union of several namespaces")
			query := fmt.Sprintf("namespaces=%s,%s", namespace1, namespace2)
			services = listServices(query)
			Expect(serviceRefsOf(services)).To(ConsistOf(
				[]string{serviceName1, namespace1},
				[]string{serviceName2, namespace2}))

			By("ignoring a requested namespace that does not exist")
			query = fmt.Sprintf("namespaces=%s,does-not-exist", namespace1)
			services = listServices(query)
			Expect(serviceRefsOf(services)).To(ConsistOf(
				[]string{serviceName1, namespace1}))

			By("returning an empty list when nothing matches")
			services = listServices("namespaces=does-not-exist")
			Expect(services).To(BeEmpty())

			By("combining the namespace filter with the search term")
			query = fmt.Sprintf("namespaces=%s,%s&search=%s",
				namespace1, namespace2, serviceName2)
			services = listServices(query)
			Expect(serviceRefsOf(services)).To(ConsistOf(
				[]string{serviceName2, namespace2}))
		})

		It("cannot widen access beyond the user's namespaces", func() {
			endpoint := fmt.Sprintf("%s%s/services?namespaces=%s",
				serverURL, v1.Root, namespace1)
			request, err := http.NewRequest(http.MethodGet, endpoint, nil)
			Expect(err).ToNot(HaveOccurred())
			request.SetBasicAuth(user, password)

			response, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			defer response.Body.Close()

			var services models.ServiceList
			err = json.NewDecoder(response.Body).Decode(&services)
			Expect(err).ToNot(HaveOccurred())
			Expect(services).To(BeEmpty())
		})
	})

	Describe("GET /api/v1/services catalog_service filter", func() {
		var serviceName1, serviceName2 string

		BeforeEach(func() {
			serviceName1 = catalog.NewServiceName()
			serviceName2 = catalog.NewServiceName()

			env.TargetNamespace(namespace1)
			env.MakeServiceInstance(serviceName1, catalogService.Meta.Name)

			env.TargetNamespace(namespace2)
			env.MakeServiceInstance(
				serviceName2,
				otherCatalogService.Meta.Name,
			)
		})

		AfterEach(func() {
			catalog.DeleteService(serviceName1, namespace1)
			catalog.DeleteService(serviceName2, namespace2)
		})

		It("returns only the instances of that catalog service", func() {
			endpoint := fmt.Sprintf("%s%s/services?catalog_service=%s",
				serverURL, v1.Root, catalogService.Meta.Name)
			response, err := env.Curl("GET", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			var serviceListResponse models.ServiceList
			err = json.NewDecoder(response.Body).Decode(&serviceListResponse)
			Expect(err).ToNot(HaveOccurred())

			names := []string{}
			for _, svc := range serviceListResponse {
				Expect(svc.CatalogService).
					To(Equal(catalogService.Meta.Name))
				names = append(names, svc.Meta.Name)
			}

			Expect(names).To(ContainElement(serviceName1))
			Expect(names).ToNot(ContainElement(serviceName2))
		})

		It("returns an empty list for an unknown catalog service", func() {
			endpoint := fmt.Sprintf(
				"%s%s/services?catalog_service=no-such-catalog",
				serverURL, v1.Root)
			response, err := env.Curl("GET", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			// [] and not null -- the dashboard parses null as a single
			// resource.
			Expect(strings.TrimSpace(string(bodyBytes))).To(Equal("[]"))
		})
	})

	Describe("GET /api/v1/namespaces/:namespace/services catalog_service filter", func() {
		var serviceName1, serviceName2 string

		BeforeEach(func() {
			serviceName1 = catalog.NewServiceName()
			serviceName2 = catalog.NewServiceName()

			env.TargetNamespace(namespace1)
			env.MakeServiceInstance(serviceName1, catalogService.Meta.Name)
			env.MakeServiceInstance(
				serviceName2,
				otherCatalogService.Meta.Name,
			)
		})

		AfterEach(func() {
			catalog.DeleteService(serviceName1, namespace1)
			catalog.DeleteService(serviceName2, namespace1)
		})

		It("returns only the instances of that catalog service", func() {
			endpoint := fmt.Sprintf(
				"%s%s/namespaces/%s/services?catalog_service=%s",
				serverURL, v1.Root, namespace1, catalogService.Meta.Name)
			response, err := env.Curl("GET", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			var serviceListResponse models.ServiceList
			err = json.NewDecoder(response.Body).Decode(&serviceListResponse)
			Expect(err).ToNot(HaveOccurred())

			Expect(serviceListResponse).To(HaveLen(1))
			Expect(serviceListResponse[0].Meta.Name).To(Equal(serviceName1))
			Expect(serviceListResponse[0].CatalogService).
				To(Equal(catalogService.Meta.Name))
		})

		It("composes with the search filter", func() {
			endpoint := fmt.Sprintf(
				"%s%s/namespaces/%s/services?catalog_service=%s&search=%s",
				serverURL, v1.Root, namespace1,
				catalogService.Meta.Name, serviceName2)
			response, err := env.Curl("GET", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			var serviceListResponse models.ServiceList
			err = json.NewDecoder(response.Body).Decode(&serviceListResponse)
			Expect(err).ToNot(HaveOccurred())

			// serviceName2 belongs to the other catalog service, so the two
			// filters cannot both be satisfied.
			Expect(serviceListResponse).To(BeEmpty())
		})
	})
})
