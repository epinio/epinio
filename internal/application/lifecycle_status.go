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

package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// NewLifecycleTiming builds a timing stamp from start/finish instants.
// durationMs is non-negative.
func NewLifecycleTiming(started, finished time.Time, stageID, result string) models.LifecycleTiming {
	started = started.UTC()
	finished = finished.UTC()
	durationMs := finished.Sub(started).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}

	return models.LifecycleTiming{
		DurationMs: durationMs,
		FinishedAt: finished.Format(time.RFC3339),
		Result:     result,
		StageID:    stageID,
	}
}

// LifecycleTimingFromJobs builds a staging timing from Job status times
// (EPINIO-773): earliest StartTime and latest CompletionTime across jobs.
// Missing CompletionTime falls back to now. Missing StartTime (or no jobs
// completed before a wait error) falls back to fallbackStart — typically
// when waitForStagingCompletion began — so a failed/timed-out stage does
// not stamp a misleading 0 ms duration.
func LifecycleTimingFromJobs(
	jobs []batchv1.Job,
	stageID, result string,
	fallbackStart time.Time,
) models.LifecycleTiming {
	var started time.Time
	var finished time.Time
	haveStart := false
	haveFinish := false

	for i := range jobs {
		st := jobs[i].Status.StartTime
		if st != nil && !st.IsZero() {
			t := st.UTC()
			if !haveStart || t.Before(started) {
				started = t
				haveStart = true
			}
		}
		ct := jobs[i].Status.CompletionTime
		if ct != nil && !ct.IsZero() {
			t := ct.UTC()
			if !haveFinish || t.After(finished) {
				finished = t
				haveFinish = true
			}
		}
	}

	if !haveFinish {
		finished = time.Now().UTC()
	}
	if !haveStart {
		if !fallbackStart.IsZero() {
			started = fallbackStart.UTC()
		} else {
			started = finished
		}
	}

	return NewLifecycleTiming(started, finished, stageID, result)
}

// LifecycleResult maps a successful/failed outcome to the CRD result value.
func LifecycleResult(succeeded bool) string {
	if succeeded {
		return models.AppLifecycleSucceeded
	}
	return models.AppLifecycleFailed
}

// stampTimeout bounds the status write for one lifecycle stamp.
const stampTimeout = 30 * time.Second

// StampContext returns the context for writing a lifecycle stamp. It keeps
// the values of ctx but not its cancellation, so the stamp still lands when
// the operation was cancelled (client gone, request timeout), which is the
// failure case the stamp exists to record. The caller must call cancel.
func StampContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), stampTimeout)
}

// LifecycleStatusMergePatch builds the merge-patch body for status.lastbuild
// or status.lastdeploy. Field must be the CRD name ("lastbuild" / "lastdeploy").
func LifecycleStatusMergePatch(field string, stamp models.LifecycleTiming) ([]byte, error) {
	return json.Marshal(map[string]any{
		"status": map[string]any{
			field: stamp,
		},
	})
}

// LastBuild reads status.lastbuild from the App CR. It returns nil when the
// field is absent or empty so the API can omit the key.
func LastBuild(app *unstructured.Unstructured) (*models.LifecycleTiming, error) {
	return lifecycleStampFromStatus(app, "lastbuild")
}

// LastDeploy reads status.lastdeploy from the App CR. It returns nil when the
// field is absent or empty so the API can omit the key.
func LastDeploy(app *unstructured.Unstructured) (*models.LifecycleTiming, error) {
	return lifecycleStampFromStatus(app, "lastdeploy")
}

func lifecycleStampFromStatus(
	app *unstructured.Unstructured,
	field string,
) (*models.LifecycleTiming, error) {
	raw, found, err := unstructured.NestedMap(
		app.UnstructuredContent(),
		"status",
		field,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "status.%s should be an object", field)
	}
	if !found || len(raw) == 0 {
		return nil, nil
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, errors.Wrapf(err, "encoding status.%s", field)
	}

	var stamp models.LifecycleTiming
	if err := json.Unmarshal(encoded, &stamp); err != nil {
		return nil, errors.Wrapf(err, "decoding status.%s", field)
	}

	if stamp.Result == "" &&
		stamp.FinishedAt == "" &&
		stamp.StageID == "" &&
		stamp.DurationMs == 0 {
		return nil, nil
	}

	return &stamp, nil
}

// SetLastBuild writes status.lastbuild on the App CR via the status subresource.
func SetLastBuild(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
	stamp models.LifecycleTiming,
) error {
	return patchAppLifecycleStatus(ctx, cluster, app, "lastbuild", stamp)
}

// SetLastDeploy writes status.lastdeploy on the App CR via the status subresource.
func SetLastDeploy(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
	stamp models.LifecycleTiming,
) error {
	return patchAppLifecycleStatus(ctx, cluster, app, "lastdeploy", stamp)
}

func patchAppLifecycleStatus(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
	field string,
	stamp models.LifecycleTiming,
) error {
	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	patch, err := LifecycleStatusMergePatch(field, stamp)
	if err != nil {
		return errors.Wrap(err, "error building status patch")
	}

	_, err = client.Namespace(app.Namespace).Patch(
		ctx,
		app.Name,
		types.MergePatchType,
		patch,
		metav1.PatchOptions{},
		"status",
	)
	return err
}
