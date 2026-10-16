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

var _ = Describe("ServiceList Endpoint, scoped to an application", LService, func() {
	var namespace, boundApp, unboundApp string
	var boundService, otherService string
	var catalogService models.CatalogService

	containerImageURL := "epinio/sample-app"

	serviceNamesFor := func(query string) []string {
		endpoint := fmt.Sprintf("%s%s/namespaces/%s/services%s",
			serverURL, v1.Root, namespace, query)

		response, err := env.Curl("GET", endpoint, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		defer response.Body.Close()

		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var serviceList models.ServiceList
		Expect(json.Unmarshal(bodyBytes, &serviceList)).ToNot(HaveOccurred())

		names := []string{}
		for _, service := range serviceList {
			names = append(names, service.Meta.Name)
		}

		return names
	}

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

		boundService = catalog.NewServiceName()
		otherService = catalog.NewServiceName()
		env.MakeServiceInstance(boundService, catalogService.Meta.Name)
		env.MakeServiceInstance(otherService, catalogService.Meta.Name)

		boundApp = catalog.NewAppName()
		unboundApp = catalog.NewAppName()
		env.MakeContainerImageApp(boundApp, 1, containerImageURL)
		env.MakeContainerImageApp(unboundApp, 1, containerImageURL)

		out, err := env.Epinio("", "service", "bind", boundService, boundApp)
		Expect(err).ToNot(HaveOccurred(), out)
	})

	AfterEach(func() {
		env.DeleteApp(boundApp)
		env.DeleteApp(unboundApp)
		catalog.DeleteService(boundService, namespace)
		catalog.DeleteService(otherService, namespace)
		catalog.DeleteCatalogService(catalogService.Meta.Name)
		env.DeleteNamespace(namespace)
	})

	It("returns every service in the namespace without the parameter", func() {
		Expect(serviceNamesFor("")).To(ConsistOf(boundService, otherService))
	})

	It("returns only the services bound to the named application", func() {
		Expect(serviceNamesFor("?app=" + boundApp)).To(ConsistOf(boundService))
	})

	It("returns an empty list for an application with no bound services", func() {
		Expect(serviceNamesFor("?app=" + unboundApp)).To(BeEmpty())
	})

	It("composes with the search filter", func() {
		Expect(serviceNamesFor(fmt.Sprintf("?app=%s&search=%s", boundApp, otherService))).
			To(BeEmpty())
	})

	It("returns 404 for an unknown application", func() {
		endpoint := fmt.Sprintf("%s%s/namespaces/%s/services?app=bogus",
			serverURL, v1.Root, namespace)

		response, err := env.Curl("GET", endpoint, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		defer response.Body.Close()

		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})
})
