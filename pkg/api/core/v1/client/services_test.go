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

var _ = Describe("Client Services", func() {
	Describe("Services Errors", DescribeServicesErrors)
})

func DescribeServicesErrors() {

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

	Describe("deleting a service", func() {
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
								"title": "service 'srv1' does not exist",
								"details": ""
							}
						]
					}`

					resp, err := epinioClient.ServiceDelete(models.ServiceDeleteRequest{}, "namespace-foo", []string{"srv1"})
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(&client.APIError{
						StatusCode: 404,
						Err: &apierrors.ErrorResponse{
							Errors: []apierrors.APIError{
								{
									Status: 404,
									Title:  "service 'srv1' does not exist",
								},
							},
						},
					}))
					Expect(resp).To(Equal(models.ServiceDeleteResponse{}))
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
			Entry("service catalog", func() (any, error) {
				return epinioClient.ServiceCatalog()
			}),
			Entry("service catalog show", func() (any, error) {
				return epinioClient.ServiceCatalogShow("servicename")
			}),
			Entry("service all services", func() (any, error) {
				return epinioClient.AllServices()
			}),
			Entry("service create", func() (any, error) {
				return epinioClient.ServiceCreate(models.ServiceCreateRequest{}, "namespace")
			}),
			Entry("service update", func() (any, error) {
				return epinioClient.ServiceUpdate(models.ServiceUpdateRequest{}, "namespace", "prefix")
			}),
			Entry("service catalog match", func() (any, error) {
				return epinioClient.ServiceCatalogMatch("servicenameprefix")
			}),
			Entry("service bind", func() (any, error) {
				return epinioClient.ServiceBind(models.ServiceBindRequest{}, "namespace", "servicename")
			}),
			Entry("service unbind", func() (any, error) {
				return epinioClient.ServiceUnbind(models.ServiceUnbindRequest{}, "namespace", "servicename")
			}),
			Entry("service list", func() (any, error) {
				return epinioClient.ServiceList("namespace")
			}),
			Entry("service show", func() (any, error) {
				return epinioClient.ServiceShow("namespace", "servicename")
			}),
			Entry("service match", func() (any, error) {
				return epinioClient.ServiceMatch("namespace", "servicenameprefix")
			}),
			Entry("service match", func() (any, error) {
				return epinioClient.ServiceDelete(models.ServiceDeleteRequest{}, "namespace", nil)
			}),
			Entry("service apps", func() (any, error) {
				return epinioClient.ServiceApps("namespace")
			}),
		)
	})
}
