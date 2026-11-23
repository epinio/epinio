// Copyright © 2021 - 2026 KRUMWARE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helmchart_test

import (
	"github.com/epinio/epinio/internal/helmchart"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("SettingsToChart", func() {
	It("includes description and example when present", func() {
		chart := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"settings": map[string]interface{}{
						"appListeningPort": map[string]interface{}{
							"type":        "integer",
							"minimum":     "0",
							"description": "Port the application listens on inside the container",
							"example":     "8080",
						},
					},
				},
			},
		}

		settings, err := helmchart.SettingsToChart(chart)
		Expect(err).ToNot(HaveOccurred())
		Expect(settings).To(HaveKey("appListeningPort"))

		port := settings["appListeningPort"]
		Expect(port.Type).To(Equal("integer"))
		Expect(port.Minimum).To(Equal("0"))
		Expect(port.Description).To(Equal(
			"Port the application listens on inside the container",
		))
		Expect(port.Example).To(Equal("8080"))
	})

	It("allows missing description and example", func() {
		chart := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"settings": map[string]interface{}{
						"httpsRedirect": map[string]interface{}{
							"type": "bool",
						},
					},
				},
			},
		}

		settings, err := helmchart.SettingsToChart(chart)
		Expect(err).ToNot(HaveOccurred())
		Expect(settings).To(HaveKey("httpsRedirect"))

		setting := settings["httpsRedirect"]
		Expect(setting.Type).To(Equal("bool"))
		Expect(setting.Description).To(BeEmpty())
		Expect(setting.Example).To(BeEmpty())
	})
})
