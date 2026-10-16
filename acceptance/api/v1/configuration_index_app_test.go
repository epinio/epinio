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

var _ = Describe("Configurations Endpoint, scoped to an application", LConfiguration, func() {
	var namespace, boundApp, unboundApp string
	var boundConfiguration, otherConfiguration string

	containerImageURL := "epinio/sample-app"

	configurationNamesFor := func(query string) []string {
		endpoint := fmt.Sprintf("%s%s/namespaces/%s/configurations%s",
			serverURL, v1.Root, namespace, query)

		response, err := env.Curl("GET", endpoint, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		defer response.Body.Close()

		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var configurationList models.ConfigurationResponseList
		Expect(json.Unmarshal(bodyBytes, &configurationList)).ToNot(HaveOccurred())

		names := []string{}
		for _, configuration := range configurationList {
			names = append(names, configuration.Meta.Name)
		}

		return names
	}

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		boundConfiguration = catalog.NewConfigurationName()
		otherConfiguration = catalog.NewConfigurationName()
		env.MakeConfiguration(boundConfiguration)
		env.MakeConfiguration(otherConfiguration)

		boundApp = catalog.NewAppName()
		unboundApp = catalog.NewAppName()
		env.MakeContainerImageApp(boundApp, 1, containerImageURL)
		env.MakeContainerImageApp(unboundApp, 1, containerImageURL)

		env.BindAppConfiguration(boundApp, boundConfiguration, namespace)
	})

	AfterEach(func() {
		env.DeleteApp(boundApp)
		env.DeleteApp(unboundApp)
		env.DeleteConfigurations(boundConfiguration, otherConfiguration)
		env.DeleteNamespace(namespace)
	})

	It("returns every configuration in the namespace without the parameter", func() {
		Expect(configurationNamesFor("")).
			To(ConsistOf(boundConfiguration, otherConfiguration))
	})

	It("returns only the configurations bound to the named application", func() {
		Expect(configurationNamesFor("?app=" + boundApp)).
			To(ConsistOf(boundConfiguration))
	})

	It("returns an empty list for an application with no bound configurations", func() {
		Expect(configurationNamesFor("?app=" + unboundApp)).To(BeEmpty())
	})

	It("composes with the search filter", func() {
		Expect(configurationNamesFor(
			fmt.Sprintf("?app=%s&search=%s", boundApp, otherConfiguration),
		)).To(BeEmpty())
	})

	It("returns 404 for an unknown application", func() {
		endpoint := fmt.Sprintf("%s%s/namespaces/%s/configurations?app=bogus",
			serverURL, v1.Root, namespace)

		response, err := env.Curl("GET", endpoint, strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		defer response.Body.Close()

		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})
})
