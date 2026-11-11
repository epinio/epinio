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

package application_test

import (
	"testing"
	"time"

	"github.com/epinio/epinio/internal/application"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSortAppCRsByName(t *testing.T) {
	crs := []unstructured.Unstructured{
		makeAppCR("zebra", "ns", time.Now()),
		makeAppCR("apple", "ns", time.Now()),
		makeAppCR("mango", "ns", time.Now()),
	}

	application.SortAppCRs(crs, "name", false)
	assertNames(t, crs, "apple", "mango", "zebra")

	application.SortAppCRs(crs, "name", true)
	assertNames(t, crs, "zebra", "mango", "apple")
}

func TestSortAppCRsByCreatedAt(t *testing.T) {
	now := time.Now()
	crs := []unstructured.Unstructured{
		makeAppCR("old", "ns", now.Add(-2*time.Hour)),
		makeAppCR("new", "ns", now),
		makeAppCR("mid", "ns", now.Add(-time.Hour)),
	}

	application.SortAppCRs(crs, "createdAt", false)
	assertNames(t, crs, "old", "mid", "new")

	application.SortAppCRs(crs, "createdAt", true)
	assertNames(t, crs, "new", "mid", "old")
}

func TestSortAppCRsUnknownFieldNoOp(t *testing.T) {
	crs := []unstructured.Unstructured{
		makeAppCR("zebra", "ns", time.Now()),
		makeAppCR("apple", "ns", time.Now()),
	}
	application.SortAppCRs(crs, "route", false)
	assertNames(t, crs, "zebra", "apple")
}

func TestSortAppCRsByBuildStatus(t *testing.T) {
	now := time.Now()
	crs := []unstructured.Unstructured{
		makeAppCRWithSpec("deployed-app", "ns", now, map[string]any{"buildstatus": "deployed"}),
		makeAppCRWithSpec("build-app", "ns", now, map[string]any{"buildstatus": "build"}),
		makeAppCRWithSpec("create-app", "ns", now, map[string]any{"buildstatus": "create"}),
	}

	application.SortAppCRs(crs, "buildStatus", false)
	assertNames(t, crs, "build-app", "create-app", "deployed-app")
}

func TestSortAppCRsByBuiltImageUrl(t *testing.T) {
	now := time.Now()
	crs := []unstructured.Unstructured{
		makeAppCRWithSpec("z-img", "ns", now, map[string]any{"builtimageurl": "registry/z"}),
		makeAppCRWithSpec("a-img", "ns", now, map[string]any{"builtimageurl": "registry/a"}),
	}

	application.SortAppCRs(crs, "builtImageUrl", false)
	assertNames(t, crs, "a-img", "z-img")
}

func TestNeedsEnrichedAppSort(t *testing.T) {
	if !application.NeedsEnrichedAppSort("status") {
		t.Fatal("status should need enrichment")
	}
	if !application.NeedsEnrichedAppSort("deploymentStatus") {
		t.Fatal("deploymentStatus should need enrichment")
	}
	if application.NeedsEnrichedAppSort("buildStatus") {
		t.Fatal("buildStatus should not need enrichment")
	}
}

func makeAppCR(name, namespace string, created time.Time) unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetCreationTimestamp(metav1.NewTime(created))
	return u
}

func makeAppCRWithSpec(name, namespace string, created time.Time, spec map[string]any) unstructured.Unstructured {
	u := makeAppCR(name, namespace, created)
	_ = unstructured.SetNestedMap(u.Object, spec, "spec")
	return u
}

func assertNames(t *testing.T, crs []unstructured.Unstructured, want ...string) {
	t.Helper()
	if len(crs) != len(want) {
		t.Fatalf("len: got %d want %d", len(crs), len(want))
	}
	for i, name := range want {
		if crs[i].GetName() != name {
			t.Fatalf("index %d: got %q want %q", i, crs[i].GetName(), name)
		}
	}
}
