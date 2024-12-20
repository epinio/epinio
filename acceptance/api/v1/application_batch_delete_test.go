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
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppBatchDelete Endpoint", LApplication, func() {
	var (
		namespace, app1, app2 string
	)
	containerImageURL := "epinio/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		for _, a := range []*string{&app1, &app2} {
			*a = catalog.NewAppName()
			env.MakeContainerImageApp(*a, 1, containerImageURL)

			configurationName := *a + "-conf"
			env.MakeConfiguration(configurationName)
			env.BindAppConfiguration(*a, configurationName, namespace)
		}
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	It("removes the applications and unbinds configurations", func() {
		responseBody := makeApplicationDeleteRequest(namespace, "", app1, app2)
		validateApplicationDeletionResponse(responseBody, app1+"-conf", app2+"-conf")
		verifyApplicationsDeleted(namespace, app1, app2)
	})
})

func makeApplicationDeleteRequest(namespace, requestBody string, applicationNames ...string) []byte {
	q := url.Values{}
	for _, a := range applicationNames {
		q.Add("applications[]", a)
	}
	URLParams := q.Encode()

	response, err := env.Curl("DELETE", fmt.Sprintf("%s%s/namespaces/%s/applications?%s",
		serverURL, api.Root, namespace, URLParams), strings.NewReader(requestBody))
	Expect(err).ToNot(HaveOccurred())
	Expect(response).ToNot(BeNil())

	defer response.Body.Close()
	bodyBytes, err := io.ReadAll(response.Body)
	Expect(err).ToNot(HaveOccurred(), string(bodyBytes))
	Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

	return bodyBytes
}

func validateApplicationDeletionResponse(bodyBytes []byte, configurationNames ...string) {
	var resp models.ApplicationDeleteResponse
	err := json.Unmarshal(bodyBytes, &resp)
	Expect(err).ToNot(HaveOccurred())
	Expect(resp.UnboundConfigurations).To(ContainElements(configurationNames))
}

func verifyApplicationsDeleted(namespace string, applicationsNames ...string) {
	// Confirm that they are now deleted
	responseGet, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/applications",
		serverURL, api.Root, namespace), strings.NewReader(""))
	Expect(err).ToNot(HaveOccurred())
	Expect(responseGet).ToNot(BeNil())
	defer responseGet.Body.Close()
	bodyBytesGet, err := io.ReadAll(responseGet.Body)
	Expect(err).ToNot(HaveOccurred())
	Expect(responseGet.StatusCode).To(Equal(http.StatusOK))

	var data models.AppList
	err = json.Unmarshal(bodyBytesGet, &data)
	Expect(err).ToNot(HaveOccurred())

	var existingApplications []string
	for _, app := range data {
		existingApplications = append(existingApplications, app.Meta.Name)
	}

	for _, a := range applicationsNames {
		Expect(existingApplications).ToNot(ContainElement(a))
	}
}
