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

package response_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/epinio/epinio/internal/api/v1/response"
)

type sortItem struct {
	name      string
	namespace string
	count     int
	created   time.Time
}

func TestSortByStringKey(t *testing.T) {
	items := []sortItem{
		{name: "zebra"},
		{name: "apple"},
		{name: "mango"},
	}

	response.SortByStringKey(items, false, func(i sortItem) string { return i.name })
	assertSortNames(t, items, "apple", "mango", "zebra")

	response.SortByStringKey(items, true, func(i sortItem) string { return i.name })
	assertSortNames(t, items, "zebra", "mango", "apple")
}

func TestSortByStringKeyNoOpWhenShort(t *testing.T) {
	items := []sortItem{{name: "only"}}
	response.SortByStringKey(items, false, func(i sortItem) string { return i.name })
	assertSortNames(t, items, "only")
}

func TestSortByIntKey(t *testing.T) {
	// Mirrors namespace appCount / configCount / serviceInstanceCount sorting.
	items := []sortItem{
		{name: "a", count: 2},
		{name: "b", count: 0},
		{name: "c", count: 5},
	}

	response.SortByIntKey(items, false, func(i sortItem) int { return i.count })
	assertSortNames(t, items, "b", "a", "c")

	response.SortByIntKey(items, true, func(i sortItem) int { return i.count })
	assertSortNames(t, items, "c", "a", "b")
}

func TestSortByTimeKey(t *testing.T) {
	now := time.Now()
	items := []sortItem{
		{name: "old", created: now.Add(-2 * time.Hour)},
		{name: "new", created: now},
		{name: "mid", created: now.Add(-time.Hour)},
	}

	response.SortByTimeKey(items, false, func(i sortItem) time.Time { return i.created })
	assertSortNames(t, items, "old", "mid", "new")

	response.SortByTimeKey(items, true, func(i sortItem) time.Time { return i.created })
	assertSortNames(t, items, "new", "mid", "old")
}

func TestApplyMetaSort(t *testing.T) {
	now := time.Now()
	items := []sortItem{
		{name: "zebra", namespace: "ns-b", created: now.Add(-time.Hour)},
		{name: "apple", namespace: "ns-a", created: now},
		{name: "mango", namespace: "ns-a", created: now.Add(-2 * time.Hour)},
	}
	keys := response.MetaKeys[sortItem]{
		Name:      func(i sortItem) string { return i.name },
		Namespace: func(i sortItem) string { return i.namespace },
		CreatedAt: func(i sortItem) time.Time { return i.created },
	}

	cloned := append([]sortItem(nil), items...)
	response.ApplyMetaSort(cloned, response.SortParams{Field: "name"}, keys)
	assertSortNames(t, cloned, "apple", "mango", "zebra")

	cloned = append([]sortItem(nil), items...)
	response.ApplyMetaSort(cloned, response.SortParams{Field: "name", Descending: true}, keys)
	assertSortNames(t, cloned, "zebra", "mango", "apple")

	cloned = append([]sortItem(nil), items...)
	response.ApplyMetaSort(cloned, response.SortParams{Field: "namespace"}, keys)
	assertSortNames(t, cloned, "apple", "mango", "zebra")

	cloned = append([]sortItem(nil), items...)
	response.ApplyMetaSort(cloned, response.SortParams{Field: "createdAt"}, keys)
	assertSortNames(t, cloned, "mango", "zebra", "apple")

	// Unknown field is a no-op (backwards compatible).
	cloned = append([]sortItem(nil), items...)
	response.ApplyMetaSort(cloned, response.SortParams{Field: "serviceInstanceCount"}, keys)
	assertSortNames(t, cloned, "zebra", "apple", "mango")

	// Empty field is a no-op.
	cloned = append([]sortItem(nil), items...)
	response.ApplyMetaSort(cloned, response.SortParams{}, keys)
	assertSortNames(t, cloned, "zebra", "apple", "mango")
}

func TestApplyMetaSortNamespaceBeforeName(t *testing.T) {
	// "dev" must sort ahead of "dev-2"; a joined "ns/name" key reverses
	// them because '-' sorts before '/'.
	items := []sortItem{
		{name: "x", namespace: "dev-2"},
		{name: "y", namespace: "dev"},
		{name: "a", namespace: "dev"},
	}
	keys := response.MetaKeys[sortItem]{
		Name:      func(i sortItem) string { return i.name },
		Namespace: func(i sortItem) string { return i.namespace },
	}

	response.ApplyMetaSort(items, response.SortParams{Field: "namespace"}, keys)
	assertSortNames(t, items, "a", "y", "x")

	response.ApplyMetaSort(
		items,
		response.SortParams{Field: "namespace", Descending: true},
		keys,
	)
	assertSortNames(t, items, "x", "y", "a")
}

func assertSortNames(t *testing.T, items []sortItem, want ...string) {
	t.Helper()
	got := make([]string, len(items))
	for i, item := range items {
		got[i] = item.name
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("names: got %v want %v", got, want)
	}
}
