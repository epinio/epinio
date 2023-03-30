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
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Namespaces API Application Endpoints", LNamespace, func() {
	var namespace string
	const jsOK = `{"status":"ok"}`

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		// Wait for server to be up and running
		Eventually(func() error {
			_, err := env.Curl("GET", serverURL+api.Root+"/info", strings.NewReader(""))
			return err
		}, "1m").ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Context("Namespaces", func() {
		Describe("GET /api/v1/namespaces", func() {
			It("lists all namespaces", func() {
				response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				var namespaces models.NamespaceList
				err = json.Unmarshal(bodyBytes, &namespaces)
				Expect(err).ToNot(HaveOccurred())

				// Reduce to relevant parts (i.e. just names, and ignoring creation time)
				namespaceNames := []string{}
				for _, n := range namespaces {
					namespaceNames = append(namespaceNames, n.Meta.Name)
				}
				// See global BeforeEach for where this namespace is set up.
				Expect(namespaceNames).Should(ContainElements(namespace))
			})
			When("basic auth credentials are not provided", func() {
				It("returns a 401 response", func() {
					request, err := http.NewRequest("GET", fmt.Sprintf("%s%s/namespaces",
						serverURL, api.Root), strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					response, err := env.Client().Do(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})
		})

		Describe("POST /api/v1/namespaces", func() {
			It("fails for non JSON body", func() {
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(``))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody).To(HaveKey("errors"), string(bodyBytes))
				Expect(responseBody["errors"][0].Title).To(Equal("EOF"))
			})

			It("fails for non-object JSON body", func() {
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`[]`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					Equal("json: cannot unmarshal array into Go value of type models.NamespaceCreateRequest"))
			})

			It("fails for JSON object without name key", func() {
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`{}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					Equal("name of namespace to create not found"))
			})

			It("fails for a known namespace", func() {
				// Create the namespace
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`{"name":"birdy"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(jsOK))

				// And the 2nd attempt should now fail
				By("creating the same namespace a second time")

				response, err = env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`{"name":"birdy"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err = io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusConflict), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					Equal("namespace 'birdy' already exists"))

				// cleanup
				env.DeleteNamespace("birdy")
			})

			It("fails for a name not fitting kubernetes requirements", func() {
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`{"name":"BOGUS"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					ContainSubstring("name must consist of lower case alphanumeric"))
			})

			It("fails for a restricted namespace", func() {
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`{"name":"epinio"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					Equal("Namespace 'epinio' name cannot be used. Please try another name"))
			})

			It("creates a new namespace", func() {
				response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces",
					serverURL, api.Root),
					strings.NewReader(`{"name":"birdwatcher"}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(jsOK))

				// cleanup
				env.DeleteNamespace("birdwatcher")
			})
		})

		Describe("GET /api/v1/namespaces/:namespace", func() {
			It("lists the namespace data", func() {
				response, err := env.Curl("GET",
					fmt.Sprintf("%s%s/namespaces/%s",
						serverURL, api.Root, namespace),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())

				var responseSpace models.Namespace
				err = json.Unmarshal(bodyBytes, &responseSpace)
				Expect(err).ToNot(HaveOccurred(), string(bodyBytes))

				// check - only relevant fields - skip: creation time
				Expect(responseSpace.Meta.Name).To(Equal(namespace))
				Expect(responseSpace.Apps).To(BeNil())
				Expect(responseSpace.Configurations).To(BeNil())
			})
		})

		Describe("GET /api/v1/namespacematches", func() {
			It("lists all namespaces for empty prefix", func() {
				response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespacematches",
					serverURL, api.Root),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				resp := models.NamespacesMatchResponse{}
				err = json.Unmarshal(bodyBytes, &resp)
				Expect(err).ToNot(HaveOccurred())

				// See global BeforeEach for where this namespace is set up.
				Expect(resp.Names).Should(ContainElements(namespace))
			})
			It("lists no namespaces matching the prefix", func() {
				response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespacematches/bogus",
					serverURL, api.Root),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				resp := models.NamespacesMatchResponse{}
				err = json.Unmarshal(bodyBytes, &resp)
				Expect(err).ToNot(HaveOccurred())

				// See global BeforeEach for where this namespace is set up.
				Expect(resp.Names).Should(BeEmpty())
			})
			It("lists all namespaces matching the prefix", func() {
				response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespacematches/na",
					serverURL, api.Root),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())
				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

				resp := models.NamespacesMatchResponse{}
				err = json.Unmarshal(bodyBytes, &resp)
				Expect(err).ToNot(HaveOccurred())

				// See global BeforeEach for where this namespace is set up.
				Expect(resp.Names).ShouldNot(BeEmpty())
			})
			When("basic auth credentials are not provided", func() {
				It("returns a 401 response", func() {
					request, err := http.NewRequest("GET", fmt.Sprintf("%s%s/namespacematches",
						serverURL, api.Root), strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					response, err := env.Client().Do(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})
		})
	})
})
