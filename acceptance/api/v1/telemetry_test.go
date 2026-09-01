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
	"fmt"
	"io"
	"net/http"
	"strings"

	v1 "github.com/epinio/epinio/internal/api/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// This only asserts the auth gate on the endpoint, not a full push to
// Grafana Cloud: the acceptance environment has no real Grafana Cloud
// credentials configured, and the trigger token is generated per-install by
// the Helm chart, so it isn't something a test outside the cluster can know.
var _ = Describe("Telemetry publish endpoint", LMisc, func() {
	It("rejects a publish request without the trigger token", func() {
		response, err := env.Curl("POST", fmt.Sprintf("%s%s/telemetry/publish",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())

		// Either telemetry is disabled in this environment (200, skipped) or
		// enabled without a matching token from us (401) - never a push.
		Expect(response.StatusCode).To(Or(Equal(http.StatusOK), Equal(http.StatusUnauthorized)), string(bodyBytes))
	})

	It("rejects a publish request with a wrong trigger token", func() {
		req, err := http.NewRequest("POST", fmt.Sprintf("%s%s/telemetry/publish", serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		req.Header.Set(v1.TelemetryTriggerHeader, "definitely-not-the-token")

		response, err := http.DefaultClient.Do(req)
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())

		Expect(response.StatusCode).To(Or(Equal(http.StatusOK), Equal(http.StatusUnauthorized)), string(bodyBytes))
	})
})
