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
	"fmt"
	"net/http"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Me endpoint", LMisc, func() {

	When("user is authenticated", func() {
		It("returns info about the current user", func() {
			bodyBytes, statusCode := me()
			Expect(statusCode).To(Equal(http.StatusOK))

			responseMe := fromJSON[models.MeResponse](bodyBytes)
			Expect(responseMe.User).To(Equal(env.EpinioUser))

			fmt.Println(string(bodyBytes))
		})
	})

	When("user is not authenticated", func() {
		It("fails getting the current user", func() {
			endpoint := makeEndpoint("me")
			request, err := http.NewRequest("GET", endpoint, nil)
			Expect(err).ToNot(HaveOccurred())

			resp, err := env.Client().Do(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})
	})
})
