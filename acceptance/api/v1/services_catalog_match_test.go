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

	v1 "github.com/epinio/epinio/internal/api/v1"
	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ServiceCatalog Match Endpoint", LService, func() {
	It("lists all catalog entries for empty prefix", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/catalogservicesmatches",
			serverURL, v1.Root),
			strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		resp := models.CatalogMatchResponse{}
		err = json.Unmarshal(bodyBytes, &resp)
		Expect(err).ToNot(HaveOccurred())

		// See global BeforeEach for where this namespace is set up.
		Expect(resp.Names).Should(ContainElements(
			"mysql-dev",
			"postgresql-dev",
			"rabbitmq-dev",
			"redis-dev",
		))
	})

	It("lists no catalog entries matching the prefix", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/catalogservicesmatches/bogus",
			serverURL, v1.Root),
			strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		resp := models.CatalogMatchResponse{}
		err = json.Unmarshal(bodyBytes, &resp)
		Expect(err).ToNot(HaveOccurred())

		// See global BeforeEach for where this namespace is set up.
		Expect(resp.Names).Should(BeEmpty())
	})

	It("lists all catalog entries matching the prefix", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/catalogservicesmatches/r",
			serverURL, v1.Root),
			strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		resp := models.CatalogMatchResponse{}
		err = json.Unmarshal(bodyBytes, &resp)
		Expect(err).ToNot(HaveOccurred())

		// See global BeforeEach for where this namespace is set up.
		Expect(resp.Names).ShouldNot(BeEmpty())
		Expect(resp.Names).Should(ContainElements(
			"rabbitmq-dev",
			"redis-dev",
		))
	})

	When("basic auth credentials are not provided", func() {
		It("returns a 401 response", func() {
			request, err := http.NewRequest("GET", fmt.Sprintf("%s%s/catalogservicesmatches",
				serverURL, v1.Root), strings.NewReader(""))
			Expect(err).ToNot(HaveOccurred())
			response, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
		})
	})
})
