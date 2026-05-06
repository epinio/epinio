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

var _ = Describe("ChartList Endpoint", LAppchart, func() {

	It("lists the known app charts", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/appcharts",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var appcharts models.AppChartList
		err = json.Unmarshal(bodyBytes, &appcharts)
		Expect(err).ToNot(HaveOccurred())

		// Note to maintainers: Due to the concurrent nature of tests we may have a varying
		// number of custom app charts from other tests visible here. Find the "standard"
		// chart in the list (order is not guaranteed) and assert on its fields.
		var standard *models.AppChart
		for i := range appcharts {
			if appcharts[i].Meta.Name == "standard" {
				standard = &appcharts[i]
				break
			}
		}
		Expect(standard).ToNot(BeNil(), "standard app chart should be present in list")

		Expect(standard.Description).To(Equal("Epinio standard support chart for application deployment"))
		Expect(standard.ShortDescription).To(Equal("Epinio standard deployment"))
		Expect(standard.HelmChart).To(MatchRegexp("https://github\\.com/epinio/helm-charts/releases/download/epinio-application-.*/epinio-application-.*\\.tgz"))
	})
})
