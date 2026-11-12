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

package response

import (
	"sort"
	"strings"
	"time"
)

// SortByStringKey sorts items by a string key before pagination.
// No-op when len(items) < 2.
func SortByStringKey[T any](items []T, descending bool, key func(T) string) {
	if len(items) < 2 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := key(items[i]), key(items[j])
		if descending {
			return a > b
		}
		return a < b
	})
}

// SortByTimeKey sorts items by a time key before pagination.
// No-op when len(items) < 2. Zero times sort as earliest.
func SortByTimeKey[T any](items []T, descending bool, key func(T) time.Time) {
	if len(items) < 2 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := key(items[i]), key(items[j])
		if descending {
			return a.After(b)
		}
		return a.Before(b)
	})
}

// SortByIntKey sorts items by an int key before pagination.
// No-op when len(items) < 2.
func SortByIntKey[T any](items []T, descending bool, key func(T) int) {
	if len(items) < 2 {
		return
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := key(items[i]), key(items[j])
		if descending {
			return a > b
		}
		return a < b
	})
}

// MetaKeys holds accessors for the common name / namespace / createdAt sort fields.
type MetaKeys[T any] struct {
	Name      func(T) string
	Namespace func(T) string // optional
	CreatedAt func(T) time.Time // optional
}

// ApplyMetaSort sorts items by SortParams using the provided meta field accessors.
// Unknown or unsupported fields are a no-op (backwards compatible).
func ApplyMetaSort[T any](items []T, params SortParams, keys MetaKeys[T]) {
	if params.Field == "" || len(items) < 2 {
		return
	}
	switch params.Field {
	case "name":
		if keys.Name == nil {
			return
		}
		SortByStringKey(items, params.Descending, func(item T) string {
			return strings.ToLower(keys.Name(item))
		})
	case "namespace":
		if keys.Namespace == nil {
			return
		}
		SortByStringKey(items, params.Descending, func(item T) string {
			ns := strings.ToLower(keys.Namespace(item))
			if keys.Name != nil {
				return ns + "/" + strings.ToLower(keys.Name(item))
			}
			return ns
		})
	case "createdAt":
		if keys.CreatedAt == nil {
			return
		}
		SortByTimeKey(items, params.Descending, keys.CreatedAt)
	}
}
