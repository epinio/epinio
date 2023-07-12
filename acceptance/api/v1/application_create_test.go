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
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/acceptance/helpers/proc"
	"github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppCreate Endpoint", LApplication, func() {
	var (
		namespace string
		appName   string
	)

	BeforeEach(func() {
		namespace = catalog.NewNamespaceName()
		appName = catalog.NewAppName()
		env.SetupAndTargetNamespace(namespace)
	})

	AfterEach(func() {
		env.DeleteNamespace(namespace)
	})

	When("creating a new app", func() {
		AfterEach(func() {
			env.DeleteApp(appName)
		})

		It("creates the app resource", func() {
			appCreateRequest := models.ApplicationCreateRequest{
				Name: appName,
				Configuration: models.ApplicationUpdateRequest{
					Routes: []string{"mytestdomain.org"},
				},
			}
			bodyBytes, statusCode := appCreate(namespace, toJSON(appCreateRequest))
			Expect(statusCode).To(Equal(http.StatusCreated), string(bodyBytes))

			out, err := proc.Kubectl("get", "apps", "-n", namespace, appName, "-o", "jsonpath={.spec.routes[*]}")
			Expect(err).ToNot(HaveOccurred(), out)
			routes := strings.Split(out, " ")
			Expect(len(routes)).To(Equal(1))
			Expect(routes[0]).To(Equal("mytestdomain.org"))
		})

		It("remembers the chart in the app resource", func() {
			appCreateRequest := models.ApplicationCreateRequest{
				Name: appName,
				Configuration: models.ApplicationUpdateRequest{
					AppChart: "standard",
				},
			}
			bodyBytes, statusCode := appCreate(namespace, toJSON(appCreateRequest))
			Expect(statusCode).To(Equal(http.StatusCreated), string(bodyBytes))

			out, err := proc.Kubectl("get", "apps", "-n", namespace, appName, "-o", "jsonpath={.spec.chartname}")
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(Equal("standard"))
		})
	})

	Describe("app creation failures", func() {
		It("fails for a name not fitting kubernetes requirements", func() {
			appCreateRequest := models.ApplicationCreateRequest{
				Name: "BOGUS",
				Configuration: models.ApplicationUpdateRequest{
					Routes: []string{"mytestdomain.org"},
				},
			}
			bodyBytes, statusCode := appCreate(namespace, toJSON(appCreateRequest))
			Expect(statusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))

			var responseBody map[string][]errors.APIError
			json.Unmarshal(bodyBytes, &responseBody)
			Expect(responseBody["errors"][0].Title).To(ContainSubstring("name must consist of lower case alphanumeric"))
		})
	})

	When("trying to create a new app with the epinio route", func() {
		It("fails creating the app", func() {
			epinioHost, err := proc.Kubectl("get", "ingress", "--namespace", "epinio", "epinio", "-o", "jsonpath={.spec.rules[*].host}")
			Expect(err).ToNot(HaveOccurred())

			appCreateRequest := models.ApplicationCreateRequest{
				Name: appName,
				Configuration: models.ApplicationUpdateRequest{
					Routes: []string{epinioHost},
				},
			}
			bodyBytes, statusCode := appCreate(namespace, toJSON(appCreateRequest))
			Expect(statusCode).To(Equal(http.StatusBadRequest), string(bodyBytes))
		})
	})
})
