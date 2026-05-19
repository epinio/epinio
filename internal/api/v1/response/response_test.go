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

package response_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/gin-gonic/gin"
)

func TestBuildPaginatedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		items        []string
		page         int
		pageSize     int
		totalItems   int
		wantPage     int
		wantPageSize int
		wantTotal    int
		wantPages    int
		wantItemsLen int
	}{
		{
			name:         "first page of three",
			items:        []string{"a", "b"},
			page: 1, pageSize: 2, totalItems: 5,
			wantPage: 1, wantPageSize: 2, wantTotal: 5, wantPages: 3, wantItemsLen: 2,
		},
		{
			name:         "last partial page",
			items:        []string{"e"},
			page: 3, pageSize: 2, totalItems: 5,
			wantPage: 3, wantPageSize: 2, wantTotal: 5, wantPages: 3, wantItemsLen: 1,
		},
		{
			name:         "empty result set floors totalPages to 1",
			items:        []string{},
			page: 1, pageSize: 10, totalItems: 0,
			wantPage: 1, wantPageSize: 10, wantTotal: 0, wantPages: 1, wantItemsLen: 0,
		},
		{
			name:         "pageSize zero returns totalPages 1",
			items:        []string{},
			page: 1, pageSize: 0, totalItems: 5,
			wantPage: 1, wantPageSize: 0, wantTotal: 5, wantPages: 1, wantItemsLen: 0,
		},
		{
			name:         "exact fit — single full page",
			items:        []string{"a", "b", "c"},
			page: 1, pageSize: 3, totalItems: 3,
			wantPage: 1, wantPageSize: 3, wantTotal: 3, wantPages: 1, wantItemsLen: 3,
		},
		{
			name:         "single item per page",
			items:        []string{"x"},
			page: 2, pageSize: 1, totalItems: 4,
			wantPage: 2, wantPageSize: 1, wantTotal: 4, wantPages: 4, wantItemsLen: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := response.BuildPaginatedResponse(tc.items, tc.page, tc.pageSize, tc.totalItems)
			if got.Page != tc.wantPage {
				t.Errorf("Page: got %d want %d", got.Page, tc.wantPage)
			}
			if got.PageSize != tc.wantPageSize {
				t.Errorf("PageSize: got %d want %d", got.PageSize, tc.wantPageSize)
			}
			if got.TotalItems != tc.wantTotal {
				t.Errorf("TotalItems: got %d want %d", got.TotalItems, tc.wantTotal)
			}
			if got.TotalPages != tc.wantPages {
				t.Errorf("TotalPages: got %d want %d", got.TotalPages, tc.wantPages)
			}
			if len(got.Items) != tc.wantItemsLen {
				t.Errorf("len(Items): got %d want %d", len(got.Items), tc.wantItemsLen)
			}
		})
	}
}

func TestGetSearchParam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	makeCtx := func(query string) *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest("GET", "/test?"+query, nil)
		c.Request = req
		return c
	}

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{name: "no search param", query: "", want: ""},
		{name: "search param with value", query: "search=foo", want: "foo"},
		{name: "empty search param", query: "search=", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := makeCtx(tc.query)
			got := response.GetSearchParam(c)
			if got != tc.want {
				t.Errorf("GetSearchParam: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestGetPaginationParams(t *testing.T) {
	gin.SetMode(gin.TestMode)

	makeCtx := func(query string) *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest("GET", "/test?"+query, nil)
		c.Request = req
		return c
	}

	tests := []struct {
		name            string
		query           string
		defaultPage     int
		defaultPageSize int
		wantPage        int
		wantPageSize    int
		wantEnabled     bool
	}{
		{
			name: "no params — disabled",
			query: "", defaultPage: 1, defaultPageSize: 25,
			wantPage: 0, wantPageSize: 0, wantEnabled: false,
		},
		{
			name: "both params present",
			query: "page=2&pageSize=10", defaultPage: 1, defaultPageSize: 25,
			wantPage: 2, wantPageSize: 10, wantEnabled: true,
		},
		{
			name: "only page present — uses default pageSize",
			query: "page=3", defaultPage: 1, defaultPageSize: 25,
			wantPage: 3, wantPageSize: 25, wantEnabled: true,
		},
		{
			name: "only pageSize present — uses default page",
			query: "pageSize=5", defaultPage: 1, defaultPageSize: 25,
			wantPage: 1, wantPageSize: 5, wantEnabled: true,
		},
		{
			name: "invalid page value — falls back to default",
			query: "page=bad&pageSize=10", defaultPage: 1, defaultPageSize: 25,
			wantPage: 1, wantPageSize: 10, wantEnabled: true,
		},
		{
			name: "zero page value — falls back to default",
			query: "page=0&pageSize=10", defaultPage: 1, defaultPageSize: 25,
			wantPage: 1, wantPageSize: 10, wantEnabled: true,
		},
		{
			name: "negative pageSize — falls back to default",
			query: "page=1&pageSize=-5", defaultPage: 1, defaultPageSize: 25,
			wantPage: 1, wantPageSize: 25, wantEnabled: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := makeCtx(tc.query)
			page, pageSize, enabled := response.GetPaginationParams(c, tc.defaultPage, tc.defaultPageSize)
			if enabled != tc.wantEnabled {
				t.Errorf("enabled: got %v want %v", enabled, tc.wantEnabled)
			}
			if page != tc.wantPage {
				t.Errorf("page: got %d want %d", page, tc.wantPage)
			}
			if pageSize != tc.wantPageSize {
				t.Errorf("pageSize: got %d want %d", pageSize, tc.wantPageSize)
			}
		})
	}
}
