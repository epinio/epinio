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

package metrics_test

import (
	"testing"

	"github.com/epinio/epinio/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestStagingAndDeploymentSeriesPreCreated(t *testing.T) {
	// init() pre-creates success/failure series so scrapes are not empty before events.
	if got := testutil.CollectAndCount(metrics.StagingCompletedTotal); got < 2 {
		t.Fatalf("expected at least success and failure staging series, got %d", got)
	}
	if got := testutil.CollectAndCount(metrics.DeploymentsTotal); got < 2 {
		t.Fatalf("expected at least success and failure deployment series, got %d", got)
	}
}

func TestRecordStagingOnceDedupesByStageID(t *testing.T) {
	stageID := t.Name()
	before := testutil.ToFloat64(metrics.StagingCompletedTotal.WithLabelValues(metrics.StatusSuccess))

	metrics.RecordStagingOnce(stageID, metrics.StatusSuccess)
	metrics.RecordStagingOnce(stageID, metrics.StatusSuccess)
	metrics.RecordStagingOnce(stageID, metrics.StatusFailure)

	after := testutil.ToFloat64(metrics.StagingCompletedTotal.WithLabelValues(metrics.StatusSuccess))
	if after != before+1 {
		t.Fatalf("expected success counter to increase by 1, before=%v after=%v", before, after)
	}
}

func TestRecordStagingOnceIgnoresUnknownStatus(t *testing.T) {
	stageID := t.Name()
	beforeSuccess := testutil.ToFloat64(metrics.StagingCompletedTotal.WithLabelValues(metrics.StatusSuccess))
	beforeFailure := testutil.ToFloat64(metrics.StagingCompletedTotal.WithLabelValues(metrics.StatusFailure))

	metrics.RecordStagingOnce(stageID, "error")

	if testutil.ToFloat64(metrics.StagingCompletedTotal.WithLabelValues(metrics.StatusSuccess)) != beforeSuccess {
		t.Fatal("success counter changed for unknown status")
	}
	if testutil.ToFloat64(metrics.StagingCompletedTotal.WithLabelValues(metrics.StatusFailure)) != beforeFailure {
		t.Fatal("failure counter changed for unknown status")
	}
}
