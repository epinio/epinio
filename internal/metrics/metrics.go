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

// Package metrics exposes Prometheus metrics for the Epinio server.
package metrics

import (
	"sync"

	"github.com/epinio/epinio/internal/version"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	StatusSuccess = "success"
	StatusFailure = "failure"
)

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "epinio_http_requests_total",
			Help: "Total number of HTTP requests handled by the Epinio server.",
		},
		[]string{"method", "route", "status"},
	)

	StagingCompletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "epinio_staging_completed_total",
			Help: "Total number of completed staging runs.",
		},
		[]string{"status"},
	)

	DeploymentsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "epinio_deployments_total",
			Help: "Total number of application deployments initiated through the API.",
		},
		[]string{"status"},
	)

	BuildInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "epinio_build_info",
			Help: "Epinio server build information.",
		},
		[]string{"version"},
	)

	// recordedStages ensures each stage ID is counted once, even if multiple
	// observers (HTTP + websocket, async deploy, etc.) wait on the same build.
	// Entries are retained for the process lifetime so late observers cannot
	// double-count; memory is one small key per completed stage ID.
	recordedStages sync.Map
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		StagingCompletedTotal,
		DeploymentsTotal,
		BuildInfo,
	)
	BuildInfo.WithLabelValues(version.Version).Set(1)

	// Pre-create labeled series so scrapes report 0 before the first event.
	for _, s := range []string{StatusSuccess, StatusFailure} {
		StagingCompletedTotal.WithLabelValues(s)
		DeploymentsTotal.WithLabelValues(s)
	}
}

// RecordStagingOnce records a staging outcome once per stage ID.
// Watch/context errors are not build outcomes and must not call this.
func RecordStagingOnce(stageID, status string) {
	if status != StatusSuccess && status != StatusFailure {
		return
	}
	if stageID != "" {
		if _, already := recordedStages.LoadOrStore(stageID, struct{}{}); already {
			return
		}
	}
	StagingCompletedTotal.WithLabelValues(status).Inc()
}

func RecordDeployment(status string) {
	if status != StatusSuccess && status != StatusFailure {
		return
	}
	DeploymentsTotal.WithLabelValues(status).Inc()
}
