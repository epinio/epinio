// Copyright © 2021 - 2026 SUSE LLC
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
	"os"

	"github.com/epinio/epinio/acceptance/helpers/catalog"
	. "github.com/epinio/epinio/acceptance/helpers/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Covers application-chart push into Epinio's configured OCI registry (bundled
// or external). Harbor-specific install coverage lives in
// acceptance/install/scenario_harbor_appcharts_test.go (EPINIO-823).
var _ = Describe("app chart push", LAppchart, func() {
	const chartArchiveURL = "https://github.com/epinio/helm-charts/releases/download/" +
		"epinio-application-0.1.26/epinio-application-0.1.26.tgz"

	var (
		chartName    string
		chartArchive string
	)

	BeforeEach(func() {
		chartName = catalog.NewTmpName("push-chart-")
		chartArchive = catalog.DownloadChartArchive(chartArchiveURL)
	})

	AfterEach(func() {
		_, _ = env.Epinio("", "app", "chart", "delete", chartName)
		if chartArchive != "" {
			_ = os.Remove(chartArchive)
		}
	})

	It("pushes a chart archive to the registry and deletes it", func() {
		By("pushing the chart")
		out, err := env.Epinio("", "app", "chart", "push", chartName, chartArchive,
			"--short-description", "pushed acceptance chart")
		Expect(err).ToNot(HaveOccurred(), out)
		Expect(out).To(ContainSubstring("Application Chart Pushed"))
		Expect(out).To(ContainSubstring(chartName))
		Expect(out).To(ContainSubstring("oci://"))
		Expect(out).To(ContainSubstring("epinio_appchart_"))

		By("showing the pushed chart")
		out, err = env.Epinio("", "app", "chart", "show", chartName)
		Expect(err).ToNot(HaveOccurred(), out)
		Expect(out).To(HaveATable(
			WithHeaders("KEY", "VALUE"),
			WithRow("Name", chartName),
			WithRow("Short", "pushed acceptance chart"),
			WithRow("Helm Repository", "oci://.*"),
			WithRow("Helm Chart", "epinio_appchart_.*"),
		))

		By("listing the pushed chart")
		out, err = env.Epinio("", "app", "chart", "list")
		Expect(err).ToNot(HaveOccurred(), out)
		Expect(out).To(ContainSubstring(chartName))

		By("deleting the pushed chart")
		out, err = env.Epinio("", "app", "chart", "delete", chartName)
		Expect(err).ToNot(HaveOccurred(), out)

		out, err = env.Epinio("", "app", "chart", "show", chartName)
		Expect(err).To(HaveOccurred(), out)
		Expect(out).To(ContainSubstring("does not exist"))
	})
})
