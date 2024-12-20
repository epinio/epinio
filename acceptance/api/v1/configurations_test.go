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
	"github.com/epinio/epinio/acceptance/helpers/proc"
	api "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configurations API Application Endpoints", LConfiguration, func() {
	containerImageURL := "epinio/sample-app"

	var namespace string
	var svc1, svc2 string

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)

		svc1 = catalog.NewConfigurationName()
		svc2 = catalog.NewConfigurationName()

		env.MakeConfiguration(svc1)
		env.MakeConfiguration(svc2)
	})

	AfterEach(func() {
		env.TargetNamespace(namespace)
		env.DeleteConfigurations(svc1)
		env.DeleteConfigurations(svc2)
		env.DeleteNamespace(namespace)
	})

	Describe("GET /api/v1/namespaces/:namespace/configurations", func() {
		var configurationNames []string

		It("lists all configurations in the namespace", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/configurations",
				serverURL, api.Root, namespace), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			var data models.ConfigurationResponseList
			err = json.Unmarshal(bodyBytes, &data)
			Expect(err).ToNot(HaveOccurred())
			configurationNames = append(configurationNames, data[0].Meta.Name)
			configurationNames = append(configurationNames, data[1].Meta.Name)
			Expect(configurationNames).Should(ContainElements(svc1, svc2))
		})

		It("returns a 404 when the namespace does not exist", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/idontexist/configurations",
				serverURL, api.Root),
				strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	Describe("GET /api/v1/configurations", func() {
		var (
			namespace1, namespace2         string
			configuration1, configuration2 string
			user, password                 string
			app1                           string
		)

		// Setting up:
		// namespace1 configuration1 app1
		// namespace2 configuration1
		// namespace2 configuration2

		BeforeEach(func() {
			namespace1 = catalog.NewNamespaceName()
			namespace2 = catalog.NewNamespaceName()
			configuration1 = catalog.NewConfigurationName()
			configuration2 = catalog.NewConfigurationName()
			app1 = catalog.NewAppName()

			env.SetupAndTargetNamespace(namespace1)
			env.MakeConfiguration(configuration1)
			env.MakeContainerImageApp(app1, 1, containerImageURL)
			env.BindAppConfiguration(app1, configuration1, namespace1)

			env.SetupAndTargetNamespace(namespace2)
			env.MakeConfiguration(configuration1) // separate from namespace1.configuration1
			env.MakeConfiguration(configuration2)

			user, password = env.CreateEpinioUser("user", nil)
		})

		AfterEach(func() {
			env.TargetNamespace(namespace2)
			env.DeleteConfigurations(configuration1)
			env.DeleteConfigurations(configuration2)

			env.TargetNamespace(namespace1)
			env.DeleteApp(app1)
			env.DeleteConfigurations(configuration1)
			env.DeleteNamespace(namespace1)
			env.DeleteNamespace(namespace2)

			env.DeleteEpinioUser(user)
		})

		It("lists all configurations belonging to all namespaces", func() {
			// But we care only about the three we know about from the setup.

			response, err := env.Curl("GET", fmt.Sprintf("%s%s/configurations",
				serverURL, api.Root), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			var configurations models.ConfigurationResponseList
			err = json.Unmarshal(bodyBytes, &configurations)
			Expect(err).ToNot(HaveOccurred())

			// `configurations` contains all configurations. Not just the two we are looking for, from
			// the setup of this test. Everything which still exists from other tests
			// executing concurrently, or not cleaned by previous tests, or the setup, or
			// ... So, we cannot be sure that the two configurations are in the two first
			// elements of the slice.

			var configurationRefs [][]string
			for _, s := range configurations {
				configurationRefs = append(configurationRefs, []string{
					s.Meta.Name,
					s.Meta.Namespace,
					strings.Join(s.Configuration.BoundApps, ", "),
				})
			}
			Expect(configurationRefs).To(ContainElements(
				[]string{configuration1, namespace1, app1},
				[]string{configuration1, namespace2, ""},
				[]string{configuration2, namespace2, ""},
			))
		})

		It("doesn't list configurations belonging to non-accessible namespaces", func() {
			endpoint := fmt.Sprintf("%s%s/configurations", serverURL, api.Root)
			request, err := http.NewRequest(http.MethodGet, endpoint, nil)
			Expect(err).ToNot(HaveOccurred())
			request.SetBasicAuth(user, password)

			response, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

			var configurations models.ConfigurationResponseList
			err = json.Unmarshal(bodyBytes, &configurations)
			Expect(err).ToNot(HaveOccurred())
			Expect(configurations).To(BeEmpty())
		})
	})

	Describe("GET /api/v1/namespaces/:namespace/configurations/:configuration", func() {
		It("lists the configuration data", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/configurations/%s",
				serverURL, api.Root, namespace, svc1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			var data models.ConfigurationResponse
			err = json.Unmarshal(bodyBytes, &data)
			configuration := data.Configuration.Details
			Expect(err).ToNot(HaveOccurred())
			Expect(configuration["username"]).To(Equal("epinio-user"))
		})

		It("returns a 404 when the namespace does not exist", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/idontexist/configurations/%s",
				serverURL, api.Root, svc1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})

		It("returns a 404 when the configuration does not exist", func() {
			response, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/configurations/bogus",
				serverURL, api.Root, namespace), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	Describe("POST /api/v1/namespaces/:namespace/configurations/:configuration", func() {
		It("fails for a name not fitting kubernetes requirements", func() {
			response, err := env.Curl("POST", fmt.Sprintf("%s%s/namespaces/%s/configurations",
				serverURL, api.Root, namespace),
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
	})

	Describe("PATCH /api/v1/namespaces/:namespace/configurations/:configuration", func() {
		var changeRequest string
		BeforeEach(func() {
			changeRequest = `{ "remove": ["username"], "edit": { "user" : "ci/cd", "host" : "up" } }`
		})

		It("edits the configuration", func() {
			// perform the editing

			response, err := env.Curl("PATCH", fmt.Sprintf("%s%s/namespaces/%s/configurations/%s",
				serverURL, api.Root, namespace, svc1), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			var responseData models.Response
			err = json.Unmarshal(bodyBytes, &responseData)
			Expect(err).ToNot(HaveOccurred())

			// then query the configuration and confirm the changes

			responseGet, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/configurations/%s",
				serverURL, api.Root, namespace, svc1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(responseGet).ToNot(BeNil())
			defer responseGet.Body.Close()
			Expect(responseGet.StatusCode).To(Equal(http.StatusOK))
			bodyBytesGet, err := io.ReadAll(responseGet.Body)
			Expect(err).ToNot(HaveOccurred())

			var data models.ConfigurationResponse
			err = json.Unmarshal(bodyBytesGet, &data)
			configuration := data.Configuration.Details

			Expect(err).ToNot(HaveOccurred())
			Expect(configuration["user"]).To(Equal("ci/cd"))
			Expect(configuration["host"]).To(Equal("up"))
			Expect(configuration).ToNot(HaveKey("username"))
		})

		It("returns a 404 when the namespace does not exist", func() {
			response, err := env.Curl("PATCH", fmt.Sprintf("%s%s/namespaces/idontexist/configurations/%s",
				serverURL, api.Root, svc1), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})

		It("returns a 404 when the configuration does not exist", func() {
			response, err := env.Curl("PATCH", fmt.Sprintf("%s%s/namespaces/%s/configurations/bogus",
				serverURL, api.Root, namespace), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})
	})

	Describe("PUT /api/v1/namespaces/:namespace/configurations/:configuration", func() {
		var changeRequest string
		BeforeEach(func() {
			changeRequest = `{ "put_key1" : "put_value" }`
		})

		It("replace the configuration", func() {
			// perform the editing

			response, err := env.Curl("PUT", fmt.Sprintf("%s%s/namespaces/%s/configurations/%s",
				serverURL, api.Root, namespace, svc1), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			var responseData models.Response
			err = json.Unmarshal(bodyBytes, &responseData)
			Expect(err).ToNot(HaveOccurred())

			// then query the configuration and confirm the changes

			responseGet, err := env.Curl("GET", fmt.Sprintf("%s%s/namespaces/%s/configurations/%s",
				serverURL, api.Root, namespace, svc1), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(responseGet).ToNot(BeNil())
			defer responseGet.Body.Close()
			Expect(responseGet.StatusCode).To(Equal(http.StatusOK))
			bodyBytesGet, err := io.ReadAll(responseGet.Body)
			Expect(err).ToNot(HaveOccurred())

			var data models.ConfigurationResponse
			err = json.Unmarshal(bodyBytesGet, &data)
			configuration := data.Configuration.Details

			Expect(err).ToNot(HaveOccurred())
			Expect(configuration["put_key1"]).To(Equal("put_value"))
			Expect(configuration).ToNot(HaveKey("username"))
		})

		It("returns a 404 when the namespace does not exist", func() {
			response, err := env.Curl("PUT", fmt.Sprintf("%s%s/namespaces/idontexist/configurations/%s",
				serverURL, api.Root, svc1), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})

		It("returns a 404 when the configuration does not exist", func() {
			response, err := env.Curl("PUT", fmt.Sprintf("%s%s/namespaces/%s/configurations/bogus",
				serverURL, api.Root, namespace), strings.NewReader(changeRequest))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
		})

		When("configuration is bound to an app", func() {
			var app1 string
			BeforeEach(func() {
				app1 = catalog.NewAppName()

				env.MakeContainerImageApp(app1, 1, containerImageURL)
				env.BindAppConfiguration(app1, svc1, namespace)
			})

			AfterEach(func() {
				env.DeleteApp(app1)
			})

			Describe("workload restarts", func() {
				It("only restarts the app if the configuration has changed", func() {
					getPodNames := func(namespace, app string) ([]string, error) {
						podName, err := proc.Kubectl("get", "pods", "-n", namespace, "-l", fmt.Sprintf("app.kubernetes.io/name=%s", app), "-o", "jsonpath='{.items[*].metadata.name}'")
						return strings.Split(podName, " "), err
					}

					oldPodNames, err := getPodNames(namespace, app1)
					Expect(err).ToNot(HaveOccurred())

					response, err := env.Curl("PUT", fmt.Sprintf("%s%s/namespaces/%s/configurations/%s",
						serverURL, api.Root, namespace, svc1), strings.NewReader(changeRequest))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())
					Expect(response.StatusCode).To(Equal(http.StatusOK))

					var newPodNames []string
					// Wait until only one pod exists (restart is finished)
					Eventually(func() int {
						newPodNames, err = getPodNames(namespace, app1)
						Expect(err).ToNot(HaveOccurred())
						return len(newPodNames)
					}, "1m").Should(Equal(1))
					Expect(newPodNames).NotTo(Equal(oldPodNames))

					// Now try with no changes
					oldPodNames, err = getPodNames(namespace, app1)
					Expect(err).ToNot(HaveOccurred())

					response, err = env.Curl("PUT", fmt.Sprintf("%s%s/namespaces/%s/configurations/%s",
						serverURL, api.Root, namespace, svc1), strings.NewReader(changeRequest))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())
					Expect(response.StatusCode).To(Equal(http.StatusOK))

					Consistently(func() []string {
						newPodNames, err := getPodNames(namespace, app1)
						Expect(err).ToNot(HaveOccurred())
						return newPodNames
					}, "10s").Should(Equal(oldPodNames))
				})
			})
		})
	})
})
