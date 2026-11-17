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
	"testing"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestStagingFailureReasonFromCR(t *testing.T) {
	app := &unstructured.Unstructured{}
	app.SetAnnotations(map[string]string{
		models.EpinioStagingFailureReasonAnnotation: "BackoffLimitExceeded",
	})

	if got := StagingFailureReason(app); got != "BackoffLimitExceeded" {
		t.Fatalf("expected BackoffLimitExceeded, got %q", got)
	}
	if got := StagingFailureReason(nil); got != "" {
		t.Fatalf("expected empty for nil app, got %q", got)
	}
	if got := StagingFailureReason(&unstructured.Unstructured{
		Object: map[string]any{"metadata": map[string]any{"name": "x"}},
	}); got != "" {
		t.Fatalf("expected empty without annotation, got %q", got)
	}
}

func TestStagingFailedStatusMessage(t *testing.T) {
	if got := stagingFailedStatusMessage(&models.App{}); got != "staging failed" {
		t.Fatalf("got %q", got)
	}
	got := stagingFailedStatusMessage(&models.App{
		StagingFailureReason: "Error (exit code 1)",
	})
	if got != "staging failed: Error (exit code 1)" {
		t.Fatalf("got %q", got)
	}
}
