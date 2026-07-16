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

package acceptance_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func appChartsURL(name string) string {
	base := strings.TrimSuffix(serverURL, "/") + "/api/v1/appcharts"
	if name == "" {
		return base
	}
	return base + "/" + name
}

var _ = Describe("AppChart CRUD", Label("appchart"), func() {
	var chartName string

	BeforeEach(func() {
		chartName = catalog.NewTmpName("appchart-")
	})

	AfterEach(func() {
		_, _ = env.Curl("DELETE", appChartsURL(chartName), nil)
	})

	It("creates, reads, updates, and deletes an appchart", func() {
		By("POST creates a new appchart")
		createBody, _ := json.Marshal(models.AppChartCreateRequest{
			Name: chartName,
			HelmChart: "https://github.com/epinio/helm-charts/" +
				"releases/download/epinio-application-0.1.26/" +
				"epinio-application-0.1.26.tgz",
			ShortDescription: "test chart",
			Description:      "long form",
		})

		createResp, createError := env.Curl(
			"POST",
			appChartsURL(""),
			bytes.NewReader(createBody),
		)
		Expect(createError).ToNot(HaveOccurred())
		decodeBody(createResp.Body, nil)
		Expect(createResp.StatusCode).To(Equal(http.StatusCreated))

		By("GET /appcharts includes the new entry")
		listResp, listError := env.Curl("GET", appChartsURL(""), nil)
		Expect(listError).ToNot(HaveOccurred())
		var list models.AppChartList
		decodeBody(listResp.Body, &list)
		Expect(listResp.StatusCode).To(Equal(http.StatusOK))
		names := []string{}
		for _, item := range list {
			names = append(names, item.Meta.Name)
		}
		Expect(names).To(ContainElement(chartName))

		By("PATCH updates the description")
		updateBody, _ := json.Marshal(models.AppChartUpdateRequest{
			ShortDescription: "updated short",
		})
		updateResp, updateError := env.Curl(
			"PATCH",
			appChartsURL(chartName),
			bytes.NewReader(updateBody),
		)
		Expect(updateError).ToNot(HaveOccurred())
		decodeBody(updateResp.Body, nil)
		Expect(updateResp.StatusCode).To(Equal(http.StatusOK))

		By("GET reflects the updated description")
		afterResp, afterError := env.Curl("GET", appChartsURL(chartName), nil)
		Expect(afterError).ToNot(HaveOccurred())
		var after models.AppChart
		decodeBody(afterResp.Body, &after)
		Expect(after.ShortDescription).To(Equal("updated short"))

		By("DELETE removes the appchart")
		deleteResp, deleteError := env.Curl(
			"DELETE",
			appChartsURL(chartName),
			nil,
		)
		Expect(deleteError).ToNot(HaveOccurred())
		decodeBody(deleteResp.Body, nil)
		Expect(deleteResp.StatusCode).To(Equal(http.StatusOK))

		By("GET after delete returns 404")
		gone, goneError := env.Curl("GET", appChartsURL(chartName), nil)
		Expect(goneError).ToNot(HaveOccurred())
		decodeBody(gone.Body, nil)
		Expect(gone.StatusCode).To(Equal(http.StatusNotFound))
	})

	It("returns 404 for PATCH against an unknown name", func() {
		body, _ := json.Marshal(models.AppChartUpdateRequest{
			ShortDescription: "x",
		})
		resp, err := env.Curl(
			"PATCH",
			appChartsURL(chartName+"-missing"),
			bytes.NewReader(body),
		)
		Expect(err).ToNot(HaveOccurred())
		decodeBody(resp.Body, nil)
		Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
	})
})
