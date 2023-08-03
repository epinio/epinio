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

package client_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/epinio/epinio/internal/cli/settings"
	"github.com/epinio/epinio/pkg/api/core/v1/client"
	apierrors "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client Configurations", func() {

	var epinioClient *client.Client
	var statusCode int
	var responseBody string

	JustBeforeEach(func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(statusCode)
			fmt.Fprint(w, responseBody)
		}))

		epinioClient = client.New(context.Background(), &settings.Settings{
			API:      srv.URL,
			Location: "fake",
		})
	})

	Describe("getting a configuration list", func() {
		Context("with a 200 status code", func() {

			BeforeEach(func() {
				statusCode = 200
			})

			When("returns an empty response", func() {
				It("gets an empty list", func() {
					responseBody = `[]`

					configs, err := epinioClient.Configurations("namespace-foo")
					Expect(err).ToNot(HaveOccurred())
					Expect(configs).To(Equal(models.ConfigurationResponseList{}))
				})
			})

			When("no JSON was returned", func() {
				It("returns an error", func() {
					responseBody = `<html>borken</html>`

					_, err := epinioClient.Configurations("namespace-foo")
					Expect(err).To(HaveOccurred())
				})
			})

			When("a valid JSON is returned", func() {
				It("returns some configurations", func() {
					responseBody = `[{},{}]`

					configs, err := epinioClient.Configurations("namespace-foo")
					Expect(err).ToNot(HaveOccurred())
					Expect(configs).To(HaveLen(2))

					resp := []models.ConfigurationResponse{
						{Meta: models.ConfigurationRef{}, Configuration: models.ConfigurationShowResponse{}},
						{Meta: models.ConfigurationRef{}, Configuration: models.ConfigurationShowResponse{}},
					}
					Expect(configs).To(Equal(models.ConfigurationResponseList(resp)))
				})
			})
		})
	})

	Describe("creating a configuration", func() {
		Context("with a 200 status code", func() {

			BeforeEach(func() {
				statusCode = 200
			})

			When("returns a valid response", func() {
				It("gets a successful response", func() {
					responseBody = `{"status":"ok"}`

					resp, err := epinioClient.ConfigurationCreate(models.ConfigurationCreateRequest{}, "namespace-foo")
					Expect(err).ToNot(HaveOccurred())
					Expect(resp).To(Equal(models.ResponseOK))
				})
			})

			When("no JSON was returned", func() {
				It("returns an error", func() {
					responseBody = `<html>borken</html>`

					_, err := epinioClient.ConfigurationCreate(models.ConfigurationCreateRequest{}, "namespace-foo")
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})

	Describe("deleting a configuration", func() {
		Context("with a 404 status code", func() {

			BeforeEach(func() {
				statusCode = 404
			})

			When("returns a valid response", func() {
				It("gets a successful response", func() {
					responseBody = `{
						"errors": [
							{
								"status": 404,
								"title": "configuration 'myconf1' does not exist",
								"details": ""
							}
						]
					}`

					resp, err := epinioClient.ConfigurationBindingDelete("namespace-foo", "app", "conf")
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(&client.APIError{
						StatusCode: 404,
						Err: &apierrors.ErrorResponse{
							Errors: []apierrors.APIError{
								{
									Status: 404,
									Title:  "configuration 'myconf1' does not exist",
								},
							},
						},
					}))
					Expect(resp).To(Equal(models.Response{}))
				})
			})
		})
	})

	When("a 500 status code and a JSON error was returned", func() {

		BeforeEach(func() {
			statusCode = 500
			responseBody = `{
					"errors": [
						{
							"status": 500,
							"title": "Error title",
							"details": "something bad happened"
						}
					]
				}`
		})

		DescribeTable("the APIs are returning an error",
			func(call func() (any, error)) {
				_, err := call()
				Expect(err).To(HaveOccurred())
			},
			Entry("configuration", func() (any, error) {
				return epinioClient.Configurations("namespace")
			}),
			Entry("all configurations", func() (any, error) {
				return epinioClient.AllConfigurations()
			}),
			Entry("configurations binding", func() (any, error) {
				return epinioClient.ConfigurationBindingCreate(models.BindRequest{}, "namespace", "app")
			}),
			Entry("configurations binding delete", func() (any, error) {
				return epinioClient.ConfigurationBindingDelete("namespace", "app", "conf")
			}),
			Entry("configurations delete", func() (any, error) {
				return epinioClient.ConfigurationDelete(models.ConfigurationDeleteRequest{}, "namespace", []string{"conf1", "conf2"})
			}),
			Entry("configuration create", func() (any, error) {
				return epinioClient.ConfigurationCreate(models.ConfigurationCreateRequest{}, "namespace")
			}),
			Entry("configuration update", func() (any, error) {
				return epinioClient.ConfigurationUpdate(models.ConfigurationUpdateRequest{}, "namespace", "prefix")
			}),
			Entry("configuration show", func() (any, error) {
				return epinioClient.ConfigurationShow("namespace", "config")
			}),
			Entry("configuration apps", func() (any, error) {
				return epinioClient.ConfigurationApps("namespace")
			}),
			Entry("configuration match", func() (any, error) {
				return epinioClient.ConfigurationMatch("namespace", "prefix")
			}),
		)
	})
})
