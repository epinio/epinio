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

package application_test

import (
	"encoding/json"
	"time"

	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("NewLifecycleTiming", func() {
	It("records a non-negative duration from start and finish", func() {
		started := time.Date(2026, 11, 9, 10, 15, 0, 0, time.UTC)
		finished := started.Add(1500 * time.Millisecond)

		stamp := application.NewLifecycleTiming(
			started,
			finished,
			"stage-abc",
			models.AppLifecycleSucceeded,
		)

		Expect(stamp.Result).To(Equal(models.AppLifecycleSucceeded))
		Expect(stamp.StageID).To(Equal("stage-abc"))
		Expect(stamp.DurationMs).To(Equal(int64(1500)))
		Expect(stamp.FinishedAt).To(Equal(finished.Format(time.RFC3339)))
	})

	It("clamps a future start time to a zero duration", func() {
		finished := time.Now().UTC()
		started := finished.Add(5 * time.Second)

		stamp := application.NewLifecycleTiming(
			started,
			finished,
			"stage-future",
			models.AppLifecycleFailed,
		)

		Expect(stamp.DurationMs).To(Equal(int64(0)))
	})
})

var _ = Describe("LifecycleTimingFromJobs", func() {
	It("uses Job StartTime and CompletionTime", func() {
		start := metav1.NewTime(time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC))
		end := metav1.NewTime(start.Add(45 * time.Second))

		stamp := application.LifecycleTimingFromJobs(
			[]batchv1.Job{{
				Status: batchv1.JobStatus{
					StartTime:      &start,
					CompletionTime: &end,
				},
			}},
			"stage-job",
			models.AppLifecycleSucceeded,
			time.Time{},
		)

		Expect(stamp.DurationMs).To(Equal(int64(45000)))
		Expect(stamp.FinishedAt).To(Equal(end.UTC().Format(time.RFC3339)))
		Expect(stamp.StageID).To(Equal("stage-job"))
		Expect(stamp.Result).To(Equal(models.AppLifecycleSucceeded))
	})

	It("spans earliest start and latest completion across jobs", func() {
		start1 := metav1.NewTime(time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC))
		end1 := metav1.NewTime(start1.Add(10 * time.Second))
		start2 := metav1.NewTime(start1.Add(5 * time.Second))
		end2 := metav1.NewTime(start1.Add(30 * time.Second))

		stamp := application.LifecycleTimingFromJobs(
			[]batchv1.Job{
				{Status: batchv1.JobStatus{StartTime: &start1, CompletionTime: &end1}},
				{Status: batchv1.JobStatus{StartTime: &start2, CompletionTime: &end2}},
			},
			"stage-multi",
			models.AppLifecycleSucceeded,
			time.Time{},
		)

		Expect(stamp.DurationMs).To(Equal(int64(30000)))
		Expect(stamp.FinishedAt).To(Equal(end2.UTC().Format(time.RFC3339)))
	})

	It("uses fallbackStart when no jobs have StartTime so duration is not zero", func() {
		fallback := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)

		stamp := application.LifecycleTimingFromJobs(
			nil,
			"stage-timeout",
			models.AppLifecycleFailed,
			fallback,
		)

		Expect(stamp.Result).To(Equal(models.AppLifecycleFailed))
		Expect(stamp.StageID).To(Equal("stage-timeout"))
		Expect(stamp.DurationMs).To(BeNumerically(">", 0))
		Expect(stamp.FinishedAt).NotTo(Equal(fallback.Format(time.RFC3339)))
	})
})

var _ = Describe("LifecycleResult", func() {
	It("maps success and failure to the CRD result values", func() {
		Expect(application.LifecycleResult(true)).To(Equal(models.AppLifecycleSucceeded))
		Expect(application.LifecycleResult(false)).To(Equal(models.AppLifecycleFailed))
	})
})

var _ = Describe("LifecycleTiming JSON", func() {
	It("marshals with CRD field names", func() {
		stamp := models.LifecycleTiming{
			DurationMs: 12345,
			FinishedAt: "2026-11-09T10:15:30Z",
			Result:     models.AppLifecycleSucceeded,
			StageID:    "stage-build-001",
		}

		raw, err := json.Marshal(stamp)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())
		Expect(got).To(HaveKey("durationms"))
		Expect(got).NotTo(HaveKey("durationMs"))
	})
})

var _ = Describe("LastBuild and LastDeploy readers", func() {
	It("returns nil when status stamps are absent", func() {
		cr := &unstructured.Unstructured{Object: map[string]any{
			"status": map[string]any{},
		}}

		build, err := application.LastBuild(cr)
		Expect(err).ToNot(HaveOccurred())
		Expect(build).To(BeNil())

		deploy, err := application.LastDeploy(cr)
		Expect(err).ToNot(HaveOccurred())
		Expect(deploy).To(BeNil())
	})

	It("reads lastbuild and lastdeploy stamps from the App CR status", func() {
		cr := &unstructured.Unstructured{Object: map[string]any{
			"status": map[string]any{
				"lastbuild": map[string]any{
					"durationms": int64(12345),
					"finishedat": "2026-11-09T10:15:30Z",
					"result":     models.AppLifecycleSucceeded,
					"stageid":    "stage-build-001",
				},
				"lastdeploy": map[string]any{
					"durationms": int64(678),
					"finishedat": "2026-11-09T10:20:00Z",
					"result":     models.AppLifecycleFailed,
					"stageid":    "stage-deploy-002",
				},
			},
		}}

		build, err := application.LastBuild(cr)
		Expect(err).ToNot(HaveOccurred())
		Expect(build).ToNot(BeNil())
		Expect(build.DurationMs).To(Equal(int64(12345)))
		Expect(build.StageID).To(Equal("stage-build-001"))

		deploy, err := application.LastDeploy(cr)
		Expect(err).ToNot(HaveOccurred())
		Expect(deploy).ToNot(BeNil())
		Expect(deploy.DurationMs).To(Equal(int64(678)))
		Expect(deploy.Result).To(Equal(models.AppLifecycleFailed))
	})
})
