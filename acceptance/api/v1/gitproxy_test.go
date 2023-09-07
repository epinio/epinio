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

	"github.com/epinio/epinio/pkg/api/core/v1/models"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gitproxy endpoint", LMisc, func() {
	It("proxies the request to github", func() {
		gitproxyRequest := models.GitProxyRequest{
			URL: "https://api.github.com/repos/epinio/epinio",
		}

		resp, statusCode := gitproxy(toJSON(gitproxyRequest))
		Expect(statusCode).To(Equal(http.StatusOK))

		var m map[string]interface{}
		err := json.Unmarshal(resp, &m)
		Expect(err).ToNot(HaveOccurred())

		Expect(m["id"]).To(BeEquivalentTo(311485110))
		Expect(m["full_name"]).To(Equal("epinio/epinio"))
	})

	It("fails for unknown endpoint", func() {
		gitproxyRequest := models.GitProxyRequest{
			URL: "https://example.com",
		}

		resp, statusCode := gitproxy(toJSON(gitproxyRequest))
		ExpectBadRequestError(resp, statusCode, "invalid proxied URL: unknown URL 'https://example.com'")
	})

	It("fails for invalid JSON", func() {
		resp, statusCode := gitproxy(strings.NewReader(`sjkskl`))
		ExpectBadRequestError(resp, statusCode, "invalid character 's' looking for beginning of value")
	})

	It("fails for non whitelisted APIs", func() {
		gitproxyRequest := models.GitProxyRequest{
			URL: "https://api.github.com/users",
		}

		resp, statusCode := gitproxy(toJSON(gitproxyRequest))
		ExpectBadRequestError(resp, statusCode, "invalid proxied URL: invalid Github URL: '/users'")
	})
})
