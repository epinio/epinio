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

package middleware_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/epinio/epinio/internal/api/v1/middleware"
	"github.com/epinio/epinio/internal/metrics"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var _ = Describe("Prometheus Middleware", func() {
	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
	})

	It("labels unmatched routes as unmatched instead of the raw path", func() {
		router := gin.New()
		router.Use(middleware.Prometheus())
		router.NoRoute(func(c *gin.Context) {
			c.Status(http.StatusNotFound)
		})

		before := testutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues("GET", "unmatched", "404"))

		req := httptest.NewRequest(http.MethodGet, "/wp-admin", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusNotFound))
		after := testutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues("GET", "unmatched", "404"))
		Expect(after).To(Equal(before + 1))

		// Raw scanner paths must not become label values.
		families, err := prometheus.DefaultGatherer.Gather()
		Expect(err).ToNot(HaveOccurred())
		for _, family := range families {
			if family.GetName() != "epinio_http_requests_total" {
				continue
			}
			for _, metric := range family.Metric {
				for _, label := range metric.Label {
					Expect(label.GetValue()).ToNot(Equal("/wp-admin"))
				}
			}
		}
	})

	It("uses the registered route template when matched", func() {
		router := gin.New()
		router.Use(middleware.Prometheus())
		router.GET("/api/v1/info", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		before := testutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues("GET", "/api/v1/info", "200"))

		req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		Expect(w.Code).To(Equal(http.StatusOK))
		after := testutil.ToFloat64(metrics.HTTPRequestsTotal.WithLabelValues("GET", "/api/v1/info", "200"))
		Expect(after).To(Equal(before + 1))
	})
})
