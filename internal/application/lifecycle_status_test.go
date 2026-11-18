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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("NewAppLifecycleStamp", func() {
	It("records a non-negative duration and RFC3339 finishedAt on success", func() {
		started := time.Now().UTC().Add(-1500 * time.Millisecond)

		stamp := application.NewAppLifecycleStamp(
			started,
			"stage-abc",
			models.AppLifecycleSucceeded,
		)

		Expect(stamp.Result).To(Equal(models.AppLifecycleSucceeded))
		Expect(stamp.StageID).To(Equal("stage-abc"))
		Expect(stamp.DurationMs).To(BeNumerically(">=", 1000))
		Expect(stamp.FinishedAt).NotTo(BeEmpty())

		_, err := time.Parse(time.RFC3339, stamp.FinishedAt)
		Expect(err).ToNot(HaveOccurred())
	})

	It("records a failed result with the same shape", func() {
		started := time.Now().UTC().Add(-200 * time.Millisecond)

		stamp := application.NewAppLifecycleStamp(
			started,
			"stage-fail",
			models.AppLifecycleFailed,
		)

		Expect(stamp.Result).To(Equal(models.AppLifecycleFailed))
		Expect(stamp.StageID).To(Equal("stage-fail"))
		Expect(stamp.DurationMs).To(BeNumerically(">=", 0))
		_, err := time.Parse(time.RFC3339, stamp.FinishedAt)
		Expect(err).ToNot(HaveOccurred())
	})

	It("clamps a future start time to a zero duration", func() {
		started := time.Now().UTC().Add(5 * time.Second)

		stamp := application.NewAppLifecycleStamp(
			started,
			"stage-future",
			models.AppLifecycleFailed,
		)

		Expect(stamp.DurationMs).To(Equal(int64(0)))
	})

	It("keeps an empty stage id when none was provided", func() {
		stamp := application.NewAppLifecycleStamp(
			time.Now().UTC(),
			"",
			models.AppLifecycleSucceeded,
		)

		Expect(stamp.StageID).To(Equal(""))
	})
})

var _ = Describe("LifecycleResult", func() {
	It("maps success and failure to the CRD result values", func() {
		Expect(application.LifecycleResult(true)).To(Equal(models.AppLifecycleSucceeded))
		Expect(application.LifecycleResult(false)).To(Equal(models.AppLifecycleFailed))
	})
})

var _ = Describe("AppLifecycleStamp JSON", func() {
	It("marshals with CRD field names", func() {
		stamp := models.AppLifecycleStamp{
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
		Expect(got).To(HaveKey("finishedat"))
		Expect(got).To(HaveKey("result"))
		Expect(got).To(HaveKey("stageid"))
		Expect(got).NotTo(HaveKey("durationMs"))
		Expect(got).NotTo(HaveKey("finishedAt"))
		Expect(got).NotTo(HaveKey("stageID"))
	})

	It("omits an empty stageid", func() {
		stamp := models.AppLifecycleStamp{
			DurationMs: 10,
			FinishedAt: "2026-11-09T10:15:30Z",
			Result:     models.AppLifecycleFailed,
		}

		raw, err := json.Marshal(stamp)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())
		Expect(got).NotTo(HaveKey("stageid"))
		Expect(got["result"]).To(Equal(models.AppLifecycleFailed))
	})
})

var _ = Describe("LifecycleStatusMergePatch", func() {
	It("builds a lastbuild status patch matching the CRD round-trip shape", func() {
		stamp := models.AppLifecycleStamp{
			DurationMs: 12345,
			FinishedAt: "2026-11-09T10:15:30Z",
			Result:     models.AppLifecycleSucceeded,
			StageID:    "stage-build-001",
		}

		raw, err := application.LifecycleStatusMergePatch("lastbuild", stamp)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())
		Expect(got).To(HaveKey("status"))

		status := got["status"].(map[string]any)
		Expect(status).To(HaveKey("lastbuild"))
		Expect(status).NotTo(HaveKey("lastdeploy"))

		lastbuild := status["lastbuild"].(map[string]any)
		Expect(lastbuild["durationms"]).To(BeEquivalentTo(12345))
		Expect(lastbuild["finishedat"]).To(Equal("2026-11-09T10:15:30Z"))
		Expect(lastbuild["result"]).To(Equal(models.AppLifecycleSucceeded))
		Expect(lastbuild["stageid"]).To(Equal("stage-build-001"))
	})

	It("builds a failed lastdeploy status patch", func() {
		stamp := models.AppLifecycleStamp{
			DurationMs: 678,
			FinishedAt: "2026-11-09T10:20:00Z",
			Result:     models.AppLifecycleFailed,
			StageID:    "stage-deploy-002",
		}

		raw, err := application.LifecycleStatusMergePatch("lastdeploy", stamp)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())

		status := got["status"].(map[string]any)
		Expect(status).To(HaveKey("lastdeploy"))
		Expect(status).NotTo(HaveKey("lastbuild"))

		lastdeploy := status["lastdeploy"].(map[string]any)
		Expect(lastdeploy["durationms"]).To(BeEquivalentTo(678))
		Expect(lastdeploy["finishedat"]).To(Equal("2026-11-09T10:20:00Z"))
		Expect(lastdeploy["result"]).To(Equal(models.AppLifecycleFailed))
		Expect(lastdeploy["stageid"]).To(Equal("stage-deploy-002"))
	})

	It("does not touch the other lifecycle field when patching one", func() {
		buildPatch, err := application.LifecycleStatusMergePatch(
			"lastbuild",
			models.AppLifecycleStamp{Result: models.AppLifecycleSucceeded},
		)
		Expect(err).ToNot(HaveOccurred())

		deployPatch, err := application.LifecycleStatusMergePatch(
			"lastdeploy",
			models.AppLifecycleStamp{Result: models.AppLifecycleFailed},
		)
		Expect(err).ToNot(HaveOccurred())

		var buildBody, deployBody map[string]any
		Expect(json.Unmarshal(buildPatch, &buildBody)).To(Succeed())
		Expect(json.Unmarshal(deployPatch, &deployBody)).To(Succeed())

		Expect(buildBody["status"].(map[string]any)).To(HaveLen(1))
		Expect(deployBody["status"].(map[string]any)).To(HaveLen(1))
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

	It("returns nil for an empty stamp object so the API omits the key", func() {
		cr := &unstructured.Unstructured{Object: map[string]any{
			"status": map[string]any{
				"lastbuild":  map[string]any{},
				"lastdeploy": map[string]any{},
			},
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
		Expect(build.FinishedAt).To(Equal("2026-11-09T10:15:30Z"))
		Expect(build.Result).To(Equal(models.AppLifecycleSucceeded))
		Expect(build.StageID).To(Equal("stage-build-001"))

		deploy, err := application.LastDeploy(cr)
		Expect(err).ToNot(HaveOccurred())
		Expect(deploy).ToNot(BeNil())
		Expect(deploy.DurationMs).To(Equal(int64(678)))
		Expect(deploy.FinishedAt).To(Equal("2026-11-09T10:20:00Z"))
		Expect(deploy.Result).To(Equal(models.AppLifecycleFailed))
		Expect(deploy.StageID).To(Equal("stage-deploy-002"))
	})

	It("omits lastBuild and lastDeploy from the App JSON when unset", func() {
		app := models.App{
			BuildStatus: models.AppBuildStatusCreate,
		}

		raw, err := json.Marshal(app)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())
		Expect(got).NotTo(HaveKey("lastBuild"))
		Expect(got).NotTo(HaveKey("lastDeploy"))
	})

	It("includes lastBuild and lastDeploy in the App JSON when set", func() {
		app := models.App{
			LastBuild: &models.AppLifecycleStamp{
				DurationMs: 100,
				FinishedAt: "2026-11-09T10:15:30Z",
				Result:     models.AppLifecycleSucceeded,
				StageID:    "stage-1",
			},
			LastDeploy: &models.AppLifecycleStamp{
				DurationMs: 50,
				FinishedAt: "2026-11-09T10:16:00Z",
				Result:     models.AppLifecycleSucceeded,
				StageID:    "stage-1",
			},
		}

		raw, err := json.Marshal(app)
		Expect(err).ToNot(HaveOccurred())

		var got map[string]any
		Expect(json.Unmarshal(raw, &got)).To(Succeed())
		Expect(got).To(HaveKey("lastBuild"))
		Expect(got).To(HaveKey("lastDeploy"))

		lastBuild := got["lastBuild"].(map[string]any)
		Expect(lastBuild["durationms"]).To(BeEquivalentTo(100))
		Expect(lastBuild).NotTo(HaveKey("durationMs"))
	})
})
