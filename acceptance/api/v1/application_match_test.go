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

var _ = Describe("ApplicationMatch Endpoint", LApplication, func() {
	var namespace string

	When("namespace doesn't exist", func() {
		It("returns an error", func() {
			endpoint := fmt.Sprintf("%s%s/namespaces/doesntexist/appsmatches",
				serverURL, v1.Root)
			response, err := env.Curl("POST", endpoint, strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusMethodNotAllowed))
		})
	})

	When("namespace exists", func() {
		var appName string

		BeforeEach(func() {
			namespace = catalog.NewNamespaceName()
			env.SetupAndTargetNamespace(namespace)

			appName = catalog.NewAppName()

			By("create it")
			out, err := env.Epinio("", "app", "create", appName)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(MatchRegexp("Ok"))
		})

		AfterEach(func() {
			env.DeleteNamespace(namespace)
		})

		It("lists all apps for empty prefix", func() {
			By("querying matches")
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/appsmatches",
				serverURL, v1.Root, namespace),
				strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			resp := models.AppMatchResponse{}
			err = json.Unmarshal(bodyBytes, &resp)
			Expect(err).ToNot(HaveOccurred())

			// See global BeforeEach for where this namespace is set up.
			Expect(resp.Names).ShouldNot(BeEmpty())
			Expect(resp.Names).Should(ContainElements(appName))
		})

		It("lists no apps matching the prefix", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/appsmatches/bogus",
				serverURL, v1.Root, namespace),
				strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			resp := models.AppMatchResponse{}
			err = json.Unmarshal(bodyBytes, &resp)
			Expect(err).ToNot(HaveOccurred())

			// See global BeforeEach for where this namespace is set up.
			Expect(resp.Names).Should(BeEmpty())
		})

		It("lists all apps matching the prefix", func() {
			By("querying matches")
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/appsmatches/%s",
				serverURL, v1.Root, namespace, appName[:len(appName)-4]),
				strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			resp := models.AppMatchResponse{}
			err = json.Unmarshal(bodyBytes, &resp)
			Expect(err).ToNot(HaveOccurred())

			// See global BeforeEach for where this namespace is set up.
			Expect(resp.Names).ShouldNot(BeEmpty())
			Expect(resp.Names).Should(ContainElements(appName))
		})

		When("basic auth credentials are not provided", func() {
			It("returns a 401 response", func() {
				request, err := http.NewRequest("GET", fmt.Sprintf("%s%s/namespaces/%s/appsmatches",
					serverURL, v1.Root, namespace), strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				response, err := env.Client().Do(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
