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

var _ = Describe("Info endpoint", LMisc, func() {
	It("includes the default builder image in the response", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/info",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var info models.InfoResponse
		err = json.Unmarshal(bodyBytes, &info)
		Expect(err).ToNot(HaveOccurred())

		Expect(info.DefaultBuilderImage).To(Equal("paketobuildpacks/builder-jammy-full:0.3.290"))
	})

	It("includes the epinio server version in a header", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/info", serverURL, v1.Root), nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		versionHeader := response.Header.Get(v1.VersionHeader)
		Expect(versionHeader).ToNot(BeEmpty())
		Expect(versionHeader).To(MatchRegexp(`v\d+\.\d+\.\d+.*`))
	})
})
