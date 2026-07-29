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

package acceptance_test

import (
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// catalogServiceURL returns the API URL for the catalogservices collection or a
// single named entry.
func catalogServiceURL(name string) string {
	base := strings.TrimSuffix(serverURL, "/") + "/api/v1/catalogservices"
	if name == "" {
		return base
	}
	return base + "/" + name
}

var _ = Describe("Catalog Service BoundServices", LService, func() {
	var namespace string
	var catalogService models.CatalogService

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		catalogService = catalog.NginxCatalogService(catalog.NewCatalogServiceName())
		catalog.CreateCatalogService(catalogService)

		DeferCleanup(func() {
			catalog.DeleteCatalogService(catalogService.Meta.Name)
			env.DeleteNamespace(namespace)
		})
	})

	It("reports BoundServices=false until an instance is provisioned, then true", func() {
		By("GET shows a freshly created catalog service has no bound instances")
		resp, err := env.Curl("GET", catalogServiceURL(catalogService.Meta.Name), nil)
		Expect(err).ToNot(HaveOccurred())
		var before models.CatalogService
		decodeBody(resp.Body, &before)
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(before.BoundServices).To(BeFalse())

		By("provisioning a service instance from the catalog service")
		serviceName := catalog.NewServiceName()
		out, err := env.Epinio("", "service", "create", catalogService.Meta.Name, serviceName, "--wait")
		Expect(err).ToNot(HaveOccurred(), out)
		DeferCleanup(func() {
			_, _ = env.Epinio("", "service", "delete", serviceName)
		})

		By("GET now reports BoundServices=true")
		Eventually(func() bool {
			resp, err := env.Curl("GET", catalogServiceURL(catalogService.Meta.Name), nil)
			Expect(err).ToNot(HaveOccurred())
			var after models.CatalogService
			decodeBody(resp.Body, &after)
			return after.BoundServices
		}, ServiceDeployTimeout, ServiceDeployPollingInterval).Should(BeTrue())
	})
})
