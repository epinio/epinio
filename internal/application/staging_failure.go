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

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// StagingFailureReason returns the staging failure detail stored on the App
// CR, or "" when none is set.
func StagingFailureReason(app *unstructured.Unstructured) string {
	if app == nil {
		return ""
	}
	return app.GetAnnotations()[models.EpinioStagingFailureReasonAnnotation]
}

// SetStagingFailureReason stores (or clears) the staging failure detail on the
// App CR. Pass an empty reason to clear a previous failure after a successful
// stage or when a new stage starts.
func SetStagingFailureReason(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	app models.AppRef,
	reason string,
) error {
	client, err := cluster.ClientApp()
	if err != nil {
		return err
	}

	var value any = reason
	if reason == "" {
		// JSON merge-patch null removes the annotation.
		value = nil
	}

	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				models.EpinioStagingFailureReasonAnnotation: value,
			},
		},
	})
	if err != nil {
		return errors.Wrap(err, "error building staging failure reason patch")
	}

	_, err = client.Namespace(app.Namespace).Patch(
		ctx,
		app.Name,
		types.MergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	return err
}

// stagingFailedStatusMessage returns the StatusMessage for a staging failure,
// including the captured reason when one is available.
func stagingFailedStatusMessage(app *models.App) string {
	if app != nil && app.StagingFailureReason != "" {
		return "staging failed: " + app.StagingFailureReason
	}
	return "staging failed"
}
