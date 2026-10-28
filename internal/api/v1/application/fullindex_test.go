// Copyright © 2021 - 2026 SUSE LLC
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
	"reflect"
	"testing"

	"github.com/epinio/epinio/internal/auth"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func appCR(name, namespace string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": name, "namespace": namespace},
	}}
}

func namesOfCRs(crs []unstructured.Unstructured) []string {
	names := []string{}
	for _, cr := range crs {
		names = append(names, cr.GetName())
	}
	return names
}

func TestFilterAppCRsByAccess(t *testing.T) {
	crs := []unstructured.Unstructured{
		appCR("app-a", "a"),
		appCR("app-b", "b"),
		appCR("app-c", "c"),
	}
	// A plain user (no admin, no global role) who owns namespaces a and b.
	user := auth.User{Namespaces: []string{"a", "b"}}

	tests := []struct {
		name       string
		namespaces []string
		want       []string
	}{
		{
			name:       "no namespace query keeps every namespace the user can see",
			namespaces: nil,
			want:       []string{"app-a", "app-b"},
		},
		{
			name:       "namespace query narrows the visible set",
			namespaces: []string{"b"},
			want:       []string{"app-b"},
		},
		{
			name:       "namespace query cannot grant access to an unowned namespace",
			namespaces: []string{"c"},
			want:       []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := namesOfCRs(filterAppCRsByAccess(user, crs, tc.namespaces))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
