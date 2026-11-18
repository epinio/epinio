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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// NewAppLifecycleStamp builds a status stamp from a start time and outcome.
// finishedAt is UTC RFC3339; durationMs is non-negative.
func NewAppLifecycleStamp(started time.Time, stageID, result string) models.AppLifecycleStamp {
	finished := time.Now().UTC()
	durationMs := finished.Sub(started).Milliseconds()
	if durationMs < 0 {
		durationMs = 0
	}

	return models.AppLifecycleStamp{
		DurationMs: durationMs,
		FinishedAt: finished.Format(time.RFC3339),
		Result:     result,
		StageID:    stageID,
	}
}

// LifecycleResult maps a successful/failed outcome to the CRD result value.
func LifecycleResult(succeeded bool) string {
	if succeeded {
		return models.AppLifecycleSucceeded
	}
	return models.AppLifecycleFailed
}

// LifecycleStatusMergePatch builds the merge-patch body for status.lastbuild
// or status.lastdeploy. Field must be the CRD name ("lastbuild" / "lastdeploy").
func LifecycleStatusMergePatch(field string, stamp models.AppLifecycleStamp) ([]byte, error) {
	return json.Marshal(map[string]any{
		"status": map[string]any{
			field: stamp,
		},
	})
}

// SetLastBuild writes status.lastbuild on the App CR via the status subresource.
func SetLastBuild(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
	stamp models.AppLifecycleStamp,
) error {
	return patchAppLifecycleStatus(ctx, cluster, app, "lastbuild", stamp)
}

// SetLastDeploy writes status.lastdeploy on the App CR via the status subresource.
func SetLastDeploy(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
	stamp models.AppLifecycleStamp,
) error {
	return patchAppLifecycleStatus(ctx, cluster, app, "lastdeploy", stamp)
}

func patchAppLifecycleStatus(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
	field string,
	stamp models.AppLifecycleStamp,
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
