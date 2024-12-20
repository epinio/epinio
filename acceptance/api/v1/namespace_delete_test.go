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
	"fmt"
	"io"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/internal/names"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DELETE /api/v1/namespaces/:namespace", LNamespace, func() {
	const jsOK = `{"status":"ok"}`
	var namespace, otherNamespace, serviceName, otherService, containerImageURL string
	var catalogService models.CatalogService

	BeforeEach(func() {
		containerImageURL = "epinio/sample-app"

		// Create a Catalog Service
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

		// Irrelevant namespace and service instance
		otherNamespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(otherNamespace)
		otherService = catalog.NewServiceName()
		env.MakeServiceInstance(otherService, catalogService.Meta.Name)

		// The namespace under test
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
		serviceName = catalog.NewServiceName()
		env.MakeServiceInstance(serviceName, catalogService.Meta.Name)

		// An app
		app1 := catalog.NewAppName()
		env.MakeContainerImageApp(app1, 1, containerImageURL)

		// A Configuration
		conf1 := catalog.NewConfigurationName()
		env.MakeConfiguration(conf1)
		env.BindAppConfiguration(app1, conf1, namespace)
	})

	AfterEach(func() {
		catalog.DeleteService(otherService, otherNamespace)
		catalog.DeleteCatalogService(catalogService.Meta.Name)
		env.DeleteNamespace(otherNamespace)
	})

	It("deletes an namespace including apps, configurations and services", func() {
		response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/%s",
			serverURL, api.Root, namespace),
			strings.NewReader(``))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(bodyBytes)).To(Equal(jsOK))

		env.VerifyNamespaceNotExist(namespace)
		out, err := proc.Kubectl("get", "secret", "-n", namespace, names.GenerateResourceName("s", serviceName))
		Expect(err).To(HaveOccurred(), out)
		Expect(out).To(MatchRegexp("not found"))

		// Doesn't delete service from other namespace
		Consistently(func() string {
			out, err := proc.Kubectl("get", "secret", "-n", otherNamespace, names.GenerateResourceName("s", otherService))
			Expect(err).ToNot(HaveOccurred(), out)
			return out
		}, "1m", "5s").Should(MatchRegexp(names.GenerateResourceName("s", otherService))) // Expect not deleted

	})
})
