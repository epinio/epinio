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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Configurations API Application Endpoints, Mutations", LConfiguration, func() {
	var namespace string
	const jsOK = `{"status":"ok"}`
	containerImageURL := "epinio/sample-app"

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	Describe("POST /api/v1/namespaces/:namespace/configurations/", func() {
		It("returns a 'bad request' for a non JSON body", func() {
			response, err := env.Curl("POST",
				fmt.Sprintf("%s%s/namespaces/%s/configurations",
					serverURL, api.Root, namespace),
				strings.NewReader(``))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			var responseBody map[string][]errors.APIError
			json.Unmarshal(bodyBytes, &responseBody)
			Expect(responseBody["errors"][0].Title).To(Equal("EOF"))
		})

		It("returns a 'bad request' for a non-object JSON body", func() {
			response, err := env.Curl("POST",
				fmt.Sprintf("%s%s/namespaces/%s/configurations",
					serverURL, api.Root, namespace),
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
				Equal("json: cannot unmarshal array into Go value of type models.ConfigurationCreateRequest"))
		})

		It("returns a 'bad request' for JSON object without `name` key", func() {
			response, err := env.Curl("POST",
				fmt.Sprintf("%s%s/namespaces/%s/configurations",
					serverURL, api.Root, namespace),
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
				Equal("cannot create configuration without a name"))
		})

		It("returns a 'not found' when the namespace does not exist", func() {
			response, err := env.Curl("POST",
				fmt.Sprintf("%s%s/namespaces/bogus/configurations",
					serverURL, api.Root),
				strings.NewReader(`{
				    "name": "meh",
				    "data": {"host":"localhost", "port":"9999"}
				}`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			var responseBody map[string][]errors.APIError
			json.Unmarshal(bodyBytes, &responseBody)
			Expect(responseBody["errors"][0].Title).To(
				Equal("namespace 'bogus' does not exist"))
		})

		Context("with conflicting configuration", func() {
			var configuration string

			BeforeEach(func() {
				configuration = catalog.NewConfigurationName()
				env.MakeConfiguration(configuration)
			})

			AfterEach(func() {
				env.CleanupConfiguration(configuration)
			})

			It("returns a 'conflict'", func() {
				response, err := env.Curl("POST",
					fmt.Sprintf("%s%s/namespaces/%s/configurations",
						serverURL, api.Root, namespace),
					strings.NewReader(fmt.Sprintf(`{
					    "name": "%s",
					    "data": {"host":"localhost", "port":"9999"}
					}`, configuration)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusConflict), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					Equal("configuration '" + configuration + "' already exists"))
			})
		})

		Describe("Creation", func() {
			var configuration string

			BeforeEach(func() {
				configuration = catalog.NewConfigurationName()
			})

			AfterEach(func() {
				env.CleanupConfiguration(configuration)
			})

			It("creates the configuration", func() {
				response, err := env.Curl("POST",
					fmt.Sprintf("%s%s/namespaces/%s/configurations",
						serverURL, api.Root, namespace),
					strings.NewReader(fmt.Sprintf(`{
					    "name": "%s",
					    "data": {"host":"localhost", "port":"9999"}
					}`, configuration)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(jsOK))
			})

			It("creates configurations without data", func() {
				response, err := env.Curl("POST",
					fmt.Sprintf("%s%s/namespaces/%s/configurations",
						serverURL, api.Root, namespace),
					strings.NewReader(`{
				    "name": "meh"
				}`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusCreated), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(jsOK))
			})
		})
	})

	Describe("DELETE /api/v1/namespaces/:namespace/configurations/:configuration", func() {
		var configuration string

		BeforeEach(func() {
			configuration = catalog.NewConfigurationName()
		})

		It("returns a 'bad request' for a non JSON body", func() {
			endpoint := fmt.Sprintf(
				"%s%s/namespaces/%s/configurations/%s",
				serverURL, api.Root, namespace, configuration,
			)
			response, err := env.Curl("DELETE", endpoint, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()

			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))

			var responseBody map[string][]errors.APIError
			json.Unmarshal(bodyBytes, &responseBody)
			Expect(responseBody["errors"][0].Title).To(Equal("EOF"))
		})

		It("returns a 'bad request' for a non-object JSON body", func() {
			endpoint := fmt.Sprintf(
				"%s%s/namespaces/%s/configurations/%s",
				serverURL, api.Root, namespace, configuration,
			)
			response, err := env.Curl("DELETE", endpoint, strings.NewReader(`[]`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))

			var responseBody map[string][]errors.APIError
			err = json.Unmarshal(bodyBytes, &responseBody)
			Expect(err).ToNot(HaveOccurred())

			expectedErrorMsg := "json: cannot unmarshal array into Go value of type models.ConfigurationDeleteRequest"
			Expect(responseBody["errors"][0].Title).To(Equal(expectedErrorMsg))
		})

		It("returns a 'not found' when the namespace does not exist", func() {
			endpoint := fmt.Sprintf(
				"%s%s/namespaces/idontexist/configurations/%s",
				serverURL, api.Root, configuration,
			)
			response, err := env.Curl("DELETE", endpoint, strings.NewReader(`{ "unbind": false }`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))

			var responseBody map[string][]errors.APIError
			err = json.Unmarshal(bodyBytes, &responseBody)
			Expect(err).ToNot(HaveOccurred())

			expectedErrorMsg := "namespace 'idontexist' does not exist"
			Expect(responseBody["errors"][0].Title).To(Equal(expectedErrorMsg))
		})

		It("returns a 'not found' when the configuration does not exist", func() {
			response, err := env.Curl("DELETE",
				fmt.Sprintf("%s%s/namespaces/%s/configurations/bogus",
					serverURL, api.Root, namespace),
				strings.NewReader(`{ "unbind": false }`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			var responseBody map[string][]errors.APIError
			json.Unmarshal(bodyBytes, &responseBody)
			Expect(responseBody["errors"][0].Title).To(
				Equal("configuration 'bogus' does not exist"))

		})

		Context("with bound applications", func() {
			var app string
			var configuration string

			BeforeEach(func() {
				configuration = catalog.NewConfigurationName()
				app = catalog.NewAppName()
				env.MakeConfiguration(configuration)
				env.MakeContainerImageApp(app, 1, containerImageURL)
				env.BindAppConfiguration(app, configuration, namespace)
			})

			AfterEach(func() {
				env.CleanupApp(app)
				env.CleanupConfiguration(configuration)
			})

			It("returns 'bad request'", func() {
				response, err := env.Curl("DELETE",
					fmt.Sprintf("%s%s/namespaces/%s/configurations/%s",
						serverURL, api.Root, namespace, configuration),
					strings.NewReader(`{ "unbind": false }`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(Equal("bound applications exist"))
				Expect(responseBody["errors"][0].Details).To(Equal(app))
			})

			It("unbinds and removes the configuration, when former is requested", func() {
				response, err := env.Curl("DELETE",
					fmt.Sprintf("%s%s/namespaces/%s/configurations/%s",
						serverURL, api.Root, namespace, configuration),
					strings.NewReader(`{ "unbind": true }`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal("{\"boundapps\":[\"" + app + "\"]}"))
			})
		})

		Context("without bound applications", func() {
			var configuration string

			BeforeEach(func() {
				configuration = catalog.NewConfigurationName()
				env.MakeConfiguration(configuration)
			})

			It("removes the configuration", func() {
				response, err := env.Curl("DELETE",
					fmt.Sprintf("%s%s/namespaces/%s/configurations/%s",
						serverURL, api.Root, namespace, configuration),
					strings.NewReader(`{ "unbind" : false }`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
				Expect(err).ToNot(HaveOccurred())
				Expect(string(bodyBytes)).To(Equal("{\"boundapps\":[]}"))
			})
		})
	})

	Describe("POST /api/v1/namespaces/:namespace/applications/:arg/configurationbindings/", func() {
		var app string

		BeforeEach(func() {
			app = catalog.NewAppName()
		})

		It("returns a 'bad request' for a non JSON body", func() {
			response, err := env.Curl("POST",
				fmt.Sprintf("%s%s/namespaces/%s/applications/%s/configurationbindings",
					serverURL, api.Root, namespace, app),
				strings.NewReader(``))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
			var responseBody map[string][]errors.APIError
			json.Unmarshal(bodyBytes, &responseBody)
			Expect(responseBody["errors"][0].Title).To(Equal("EOF"))
		})

		It("returns a 'bad request' for a non-object JSON body", func() {
			response, err := env.Curl("POST",
				fmt.Sprintf("%s%s/namespaces/%s/applications/%s/configurationbindings",
					serverURL, api.Root, namespace, app),
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
				Equal("json: cannot unmarshal array into Go value of type models.BindRequest"))
		})

		It("returns a 'bad request' for JSON object without `name` key", func() {
			response, err := env.Curl("POST",
				fmt.Sprintf("%s%s/namespaces/%s/applications/%s/configurationbindings",
					serverURL, api.Root, namespace, app),
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
				Equal("cannot bind configuration without names"))
		})

		It("returns a 'not found' when the namespace does not exist", func() {
			response, err := env.Curl("POST",
				fmt.Sprintf("%s%s/namespaces/bogus/applications/_dummy_/configurationbindings",
					serverURL, api.Root),
				strings.NewReader(`{ "names": ["meh"] }`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			var responseBody map[string][]errors.APIError
			json.Unmarshal(bodyBytes, &responseBody)
			Expect(responseBody["errors"][0].Title).To(
				Equal("namespace 'bogus' does not exist"))
		})

		It("returns a 'not found' when the application does not exist", func() {
			response, err := env.Curl("POST",
				fmt.Sprintf("%s%s/namespaces/%s/applications/bogus/configurationbindings",
					serverURL, api.Root, namespace),
				strings.NewReader(`{ "names": ["meh"] }`))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			var responseBody map[string][]errors.APIError
			json.Unmarshal(bodyBytes, &responseBody)
			Expect(responseBody["errors"][0].Title).To(
				Equal("application 'bogus' does not exist"))
		})

		Context("with application", func() {
			var app string
			var configuration string

			BeforeEach(func() {
				app = catalog.NewAppName()
				configuration = catalog.NewConfigurationName()
				env.MakeContainerImageApp(app, 1, containerImageURL)
				env.MakeConfiguration(configuration)
			})

			AfterEach(func() {
				env.CleanupApp(app)
				env.CleanupConfiguration(configuration)
			})

			It("returns a 'not found' when the configuration does not exist", func() {
				response, err := env.Curl("POST",
					fmt.Sprintf("%s%s/namespaces/%s/applications/%s/configurationbindings",
						serverURL, api.Root, namespace, app),
					strings.NewReader(`{ "names": ["bogus"] }`))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					Equal("configuration 'bogus' does not exist"))
			})

			Context("and already bound", func() {
				BeforeEach(func() {
					env.BindAppConfiguration(app, configuration, namespace)
				})

				It("returns a note about being bound", func() {
					response, err := env.Curl("POST",
						fmt.Sprintf("%s%s/namespaces/%s/applications/%s/configurationbindings",
							serverURL, api.Root, namespace, app),
						strings.NewReader(fmt.Sprintf(`{ "names": ["%s"] }`, configuration)))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())

					defer response.Body.Close()
					bodyBytes, err := io.ReadAll(response.Body)
					Expect(err).ToNot(HaveOccurred())
					Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
					var responseBody map[string][]errors.APIError
					json.Unmarshal(bodyBytes, &responseBody)
					Expect(string(bodyBytes)).To(Equal(fmt.Sprintf(`{"wasbound":["%s"]}`, configuration)))
				})
			})

			It("binds the configuration", func() {
				response, err := env.Curl("POST",
					fmt.Sprintf("%s%s/namespaces/%s/applications/%s/configurationbindings",
						serverURL, api.Root, namespace, app),
					strings.NewReader(fmt.Sprintf(`{ "names": ["%s"] }`, configuration)))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
				Expect(string(bodyBytes)).To(Equal(`{"wasbound":null}`))
			})
		})
	})

	Describe("DELETE /api/v1/namespaces/:namespace/applications/:app/configurationbindings/:configuration", func() {
		var app string
		var configuration string

		BeforeEach(func() {
			configuration = catalog.NewConfigurationName()
			app = catalog.NewAppName()
		})

		It("returns a 'not found' when the namespace does not exist", func() {
			response, err := env.Curl("DELETE",
				fmt.Sprintf("%s%s/namespaces/idontexist/applications/%s/configurationbindings/%s",
					serverURL, api.Root, app, configuration),
				strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))

			var responseBody map[string][]errors.APIError
			json.Unmarshal(bodyBytes, &responseBody)
			Expect(responseBody["errors"][0].Title).To(
				Equal("namespace 'idontexist' does not exist"))
		})

		It("returns a 'not found' when the application does not exist", func() {
			// This requires a valid configuration
			env.MakeConfiguration(configuration)
			defer env.CleanupConfiguration(configuration)

			response, err := env.Curl("DELETE",
				fmt.Sprintf("%s%s/namespaces/%s/applications/bogus/configurationbindings/%s",
					serverURL, api.Root, namespace, configuration),
				strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())

			defer response.Body.Close()
			bodyBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
			var responseBody map[string][]errors.APIError
			json.Unmarshal(bodyBytes, &responseBody)
			Expect(responseBody["errors"][0].Title).To(
				Equal("application 'bogus' does not exist"))
		})

		Context("with application", func() {
			var app string

			BeforeEach(func() {
				app = catalog.NewAppName()
				env.MakeContainerImageApp(app, 1, containerImageURL)
			})

			AfterEach(func() {
				env.CleanupApp(app)
			})

			It("returns a 'not found' when the configuration does not exist", func() {
				response, err := env.Curl("DELETE",
					fmt.Sprintf("%s%s/namespaces/%s/applications/%s/configurationbindings/bogus",
						serverURL, api.Root, namespace, app),
					strings.NewReader(""))
				Expect(err).ToNot(HaveOccurred())
				Expect(response).ToNot(BeNil())

				defer response.Body.Close()
				bodyBytes, err := io.ReadAll(response.Body)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
				var responseBody map[string][]errors.APIError
				json.Unmarshal(bodyBytes, &responseBody)
				Expect(responseBody["errors"][0].Title).To(
					Equal("configuration 'bogus' does not exist"))
			})

			Context("with configuration", func() {
				var configuration string

				BeforeEach(func() {
					configuration = catalog.NewConfigurationName()
					env.MakeConfiguration(configuration)
				})

				AfterEach(func() {
					env.CleanupConfiguration(configuration)
				})

				Context("already bound", func() {
					BeforeEach(func() {
						env.BindAppConfiguration(app, configuration, namespace)
					})

					It("unbinds the configuration", func() {
						response, err := env.Curl("DELETE",
							fmt.Sprintf("%s%s/namespaces/%s/applications/%s/configurationbindings/%s",
								serverURL, api.Root, namespace, app, configuration),
							strings.NewReader(""))
						Expect(err).ToNot(HaveOccurred())
						Expect(response).ToNot(BeNil())

						defer response.Body.Close()
						bodyBytes, err := io.ReadAll(response.Body)
						Expect(err).ToNot(HaveOccurred())
						Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
						Expect(string(bodyBytes)).To(Equal(jsOK))
					})
				})

				It("returns a 'ok' even when the configuration is not bound (idempotency)", func() {
					response, err := env.Curl("DELETE",
						fmt.Sprintf("%s%s/namespaces/%s/applications/%s/configurationbindings/%s",
							serverURL, api.Root, namespace, app, configuration),
						strings.NewReader(""))
					Expect(err).ToNot(HaveOccurred())
					Expect(response).ToNot(BeNil())

					defer response.Body.Close()
					bodyBytes, err := io.ReadAll(response.Body)
					Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))
					Expect(string(bodyBytes)).To(Equal(jsOK))
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})
})
