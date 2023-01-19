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

var _ = Describe("ChartShow Endpoint", LAppchart, func() {
	It("lists the details of the app chart", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/appcharts/standard",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())
		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK), string(bodyBytes))

		var appchart models.AppChart
		err = json.Unmarshal(bodyBytes, &appchart)
		Expect(err).ToNot(HaveOccurred())

		Expect(appchart.Meta.Name).To(Equal("standard"))
		Expect(appchart.Description).To(Equal("Epinio standard support chart for application deployment"))
		Expect(appchart.ShortDescription).To(Equal("Epinio standard deployment"))
		Expect(appchart.HelmChart).To(MatchRegexp("https://github\\.com/epinio/helm-charts/releases/download/epinio-application-.*/epinio-application-.*\\.tgz"))
	})

	It("returns a 404 when the chart does not exist", func() {
		response, err := env.Curl("GET", fmt.Sprintf("%s%s/appcharts/bogus",
			serverURL, v1.Root), strings.NewReader(""))
		Expect(err).ToNot(HaveOccurred())
		Expect(response).ToNot(BeNil())

		defer response.Body.Close()
		bodyBytes, err := io.ReadAll(response.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusNotFound), string(bodyBytes))
	})
})
