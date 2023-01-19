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

var _ = Describe("ServiceCatalog Endpoint", LService, func() {
	var catalogService models.CatalogService

	catalogResponse := func() models.CatalogServices {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/catalogservices", serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var result models.CatalogServices
		err = json.Unmarshal(bodyBytes, &result)
		Expect(err).ToNot(HaveOccurred(), string(bodyBytes))

		return result
	}

	BeforeEach(func() {
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
	})

	It("lists services from the 'epinio' namespace", func() {
		catalog.CreateCatalogService(catalogService)
		defer catalog.DeleteCatalogService(catalogService.Meta.Name)

		catalog := catalogResponse()
		serviceNames := []string{}
		for _, s := range catalog {
			serviceNames = append(serviceNames, s.Meta.Name)
		}
		Expect(serviceNames).To(ContainElement(catalogService.Meta.Name))
	})

	It("doesn't list services from namespaces other than 'epinio'", func() {
		catalog.CreateCatalogServiceInNamespace("default", catalogService)
		defer catalog.DeleteCatalogServiceFromNamespace("default", catalogService.Meta.Name)

		catalog := catalogResponse()
		serviceNames := []string{}
		for _, s := range catalog {
			serviceNames = append(serviceNames, s.Meta.Name)
		}
		Expect(serviceNames).ToNot(ContainElement(catalogService.Meta.Name))
	})
})
