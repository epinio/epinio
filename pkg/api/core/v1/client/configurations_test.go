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

		epinioClient = client.New(context.Background(), &settings.Settings{API: srv.URL})
	})

	Describe("getting a configuration list", func() {
		When("a 200 status code occurred with empty response", func() {

			BeforeEach(func() {
				statusCode = 200
				responseBody = `[]`
			})

			It("gets an empty list", func() {
				configs, err := epinioClient.Configurations("namespace-foo")
				Expect(err).ToNot(HaveOccurred())
				Expect(configs).To(Equal(models.ConfigurationResponseList{}))
			})
		})

		When("a 200 status code occurred but no JSON was returned", func() {

			BeforeEach(func() {
				statusCode = 200
				responseBody = `<html>borken</html>`
			})

			It("returns an error", func() {
				_, err := epinioClient.Configurations("namespace-foo")
				Expect(err).To(HaveOccurred())
			})
		})

		When("a 200 status code occurred with a valid JSON", func() {

			BeforeEach(func() {
				statusCode = 200
				responseBody = `[{},{}]`
			})

			It("returns some configurations", func() {
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
			Entry("configuration", func() (any, error) { return epinioClient.Configurations("namespace") }),
			Entry("all configurations", func() (any, error) { return epinioClient.AllConfigurations() }),
			Entry("configuration show", func() (any, error) { return epinioClient.ConfigurationShow("namespace", "config") }),
			Entry("configuration apps", func() (any, error) { return epinioClient.ConfigurationApps("namespace") }),
			Entry("configuration match", func() (any, error) { return epinioClient.ConfigurationMatch("namespace", "prefix") }),
		)
	})
})
