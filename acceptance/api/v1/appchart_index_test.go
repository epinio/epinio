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
		// number of custom app charts from other tests visible here. In other words, while
		// we can reliably test for the presence of the standard chart, the number of charts
		// to expect is not checkable.
		//
		// A check like `Expect(len(appcharts)).To(Equal(1))` will introduce flakiness and
		// spurious failures.

		var names []string
		names = append(names, appcharts[0].Meta.Name)
		var desc []string
		desc = append(desc, appcharts[0].Description)
		var short []string
		short = append(short, appcharts[0].ShortDescription)
		var chart []string
		chart = append(chart, appcharts[0].HelmChart)

		Expect(names).Should(ContainElements(
			"standard"))
		Expect(desc).Should(ContainElements(
			"Epinio standard support chart for application deployment"))
		Expect(short).Should(ContainElements(
			"Epinio standard deployment"))
		Expect(chart).Should(ContainElements(
			"https://github.com/epinio/helm-charts/releases/download/epinio-application-0.1.26/epinio-application-0.1.26.tgz"))
	})
})
