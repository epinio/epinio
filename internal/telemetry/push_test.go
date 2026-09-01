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

package telemetry_test

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/epinio/epinio/internal/telemetry"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/proto"
)

// gaugeValue returns the single data point value and its attributes for the
// named gauge metric found anywhere in the decoded export request.
func gaugeValue(req *colmetricpb.ExportMetricsServiceRequest, name string) (float64, map[string]string, bool) {
	for _, rm := range req.ResourceMetrics {
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.GetName() != name {
					continue
				}
				dps := m.GetGauge().GetDataPoints()
				if len(dps) == 0 {
					return 0, nil, true
				}
				attrs := map[string]string{}
				for _, kv := range dps[0].GetAttributes() {
					attrs[kv.GetKey()] = kv.GetValue().GetStringValue()
				}
				return dps[0].GetAsDouble(), attrs, true
			}
		}
	}
	return 0, nil, false
}

var _ = Describe("Push", func() {
	var snapshot telemetry.Snapshot

	BeforeEach(func() {
		snapshot = telemetry.Snapshot{
			EpinioVersion: "v1.14.1",
			ChartVersion:  "v1.14.1",
			KubeVersion:   "v1.30.0",
			Platform:      "k3s",
			InstanceID:    "instance-under-test",
			InstallMethod: "helm",
			Applications:  3,
			Namespaces:    2,
			Services:      1,
		}
	})

	When("required configuration is missing", func() {
		It("fails fast without an OTLP endpoint", func() {
			err := telemetry.Push(context.Background(), snapshot, telemetry.Config{
				GrafanaInstanceID: "123",
				GrafanaToken:      "glc_test",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("otlp endpoint"))
		})

		It("fails fast without Grafana Cloud credentials", func() {
			err := telemetry.Push(context.Background(), snapshot, telemetry.Config{
				OTLPEndpoint: "http://127.0.0.1:1/otlp/v1/metrics",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("credentials"))
		})
	})

	When("configuration is complete", func() {
		var (
			server        *httptest.Server
			receivedReq   *http.Request
			receivedBody  []byte
			exportRequest *colmetricpb.ExportMetricsServiceRequest
		)

		BeforeEach(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedReq = r.Clone(r.Context())
				body, err := io.ReadAll(r.Body)
				Expect(err).ToNot(HaveOccurred())
				receivedBody = body

				exportRequest = &colmetricpb.ExportMetricsServiceRequest{}
				Expect(proto.Unmarshal(body, exportRequest)).To(Succeed())

				w.Header().Set("Content-Type", "application/x-protobuf")
				w.WriteHeader(http.StatusOK)
				resp, err := proto.Marshal(&colmetricpb.ExportMetricsServiceResponse{})
				Expect(err).ToNot(HaveOccurred())
				_, _ = w.Write(resp)
			}))
		})

		AfterEach(func() {
			server.Close()
		})

		It("pushes the snapshot as OTLP gauges with Basic auth", func() {
			err := telemetry.Push(context.Background(), snapshot, telemetry.Config{
				OTLPEndpoint:      server.URL + "/custom/v1/metrics",
				GrafanaInstanceID: "1807358",
				GrafanaToken:      "glc_test_token",
				ClusterLabel:      "minikube-epinio",
				EnvironmentLabel:  "dev",
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(receivedReq).ToNot(BeNil())
			Expect(receivedReq.Method).To(Equal(http.MethodPost))
			Expect(receivedReq.URL.Path).To(Equal("/custom/v1/metrics"))

			authHeader := receivedReq.Header.Get("Authorization")
			Expect(authHeader).To(HavePrefix("Basic "))
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
			Expect(err).ToNot(HaveOccurred())
			Expect(string(decoded)).To(Equal("1807358:glc_test_token"))

			Expect(exportRequest).ToNot(BeNil())
			Expect(receivedBody).ToNot(BeEmpty())

			buildInfo, buildAttrs, found := gaugeValue(exportRequest, "epinio_build_info")
			Expect(found).To(BeTrue())
			Expect(buildInfo).To(Equal(1.0))
			Expect(buildAttrs).To(HaveKeyWithValue("epinio_version", "1.14.1"))
			Expect(buildAttrs).To(HaveKeyWithValue("kubernetes_version", "v1.30.0"))
			Expect(buildAttrs).To(HaveKeyWithValue("instance_id", "instance-under-test"))
			Expect(buildAttrs).To(HaveKeyWithValue("install_method", "helm"))
			Expect(buildAttrs).To(HaveKeyWithValue("cluster", "minikube-epinio"))
			Expect(buildAttrs).To(HaveKeyWithValue("environment", "dev"))

			apps, _, found := gaugeValue(exportRequest, "epinio_inventory_applications")
			Expect(found).To(BeTrue())
			Expect(apps).To(Equal(3.0))

			nss, _, found := gaugeValue(exportRequest, "epinio_inventory_namespaces")
			Expect(found).To(BeTrue())
			Expect(nss).To(Equal(2.0))

			svcs, _, found := gaugeValue(exportRequest, "epinio_inventory_services")
			Expect(found).To(BeTrue())
			Expect(svcs).To(Equal(1.0))

			lastSuccess, _, found := gaugeValue(exportRequest, "epinio_telemetry_last_success_timestamp")
			Expect(found).To(BeTrue())
			Expect(lastSuccess).To(BeNumerically(">", 0))
		})
	})

	When("the OTLP endpoint is unreachable", func() {
		It("returns an error instead of hanging", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 0)
			defer cancel()

			err := telemetry.Push(ctx, snapshot, telemetry.Config{
				OTLPEndpoint:      "http://127.0.0.1:1/v1/metrics",
				GrafanaInstanceID: "123",
				GrafanaToken:      "glc_test",
			})
			Expect(err).To(HaveOccurred())
		})
	})
})
