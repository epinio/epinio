// Copyright © 2026 SUSE LLC
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

var _ = Describe("Report endpoint", LMisc, func() {
	It("returns a report JSON payload", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/report/nodes", serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var report models.ReportResponse
		err = json.Unmarshal(bodyBytes, &report)
		Expect(err).ToNot(HaveOccurred())

		Expect(report.EpinioVersion).ToNot(BeEmpty())
		Expect(report.KubernetesVersion).ToNot(BeEmpty())
		Expect(report.Platform).ToNot(BeEmpty())
		Expect(report.Clusters).ToNot(BeEmpty())
		Expect(report.TotalNodeCount).To(BeNumerically(">=", 0))
	})

	It("returns a text report when requested", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/report/nodes?format=text", serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		body := string(bodyBytes)
		Expect(body).To(ContainSubstring("Epinio Systems Summary Report"))
		Expect(body).To(ContainSubstring("Total node count:"))
	})
})
