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

package application

import (
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ListFilter selects which applications a list operation returns, based on
// buildstatus (see IsUndeployed).
type ListFilter int

const (
	// ListFilterAll returns every application CR.
	ListFilterAll ListFilter = iota
	// ListFilterDeployed excludes apps whose buildstatus is create or build.
	// Used by the regular applications list endpoints (deployed status only).
	ListFilterDeployed
	// ListFilterUndeployed returns apps whose buildstatus is create or build
	// (including a previously deployed app that was later rebuilt build-only).
	// Used by the undeployed/builds list endpoints.
	ListFilterUndeployed
)

// IsUndeployed reports whether the app should appear on undeployed/builds
// lists: buildstatus is "create" or "build". A previously deployed app that
// is rebuilt build-only is included here while status remains "build".
func IsUndeployed(app models.App) bool {
	return isCreateOrBuildStatus(app.BuildStatus)
}

// IsUndeployedCR reports whether the App CR should appear on undeployed/builds
// lists, based on spec.buildstatus. Missing or unreadable fields are treated
// as not undeployed.
func IsUndeployedCR(app *unstructured.Unstructured) bool {
	status, err := BuildStatus(app)
	if err != nil {
		return false
	}
	return isCreateOrBuildStatus(status)
}

// isCreateOrBuildStatus is true when buildstatus is create or build.
func isCreateOrBuildStatus(buildStatus string) bool {
	return buildStatus == models.AppBuildStatusCreate ||
		buildStatus == models.AppBuildStatusBuild
}

// MatchesListFilter reports whether the App CR matches the given ListFilter.
func MatchesListFilter(cr *unstructured.Unstructured, filter ListFilter) bool {
	return appCRMatchesFilter(cr, filter)
}

// FilterApps keeps apps that match the given ListFilter.
func FilterApps(apps models.AppList, filter ListFilter) models.AppList {
	if filter == ListFilterAll || len(apps) == 0 {
		return apps
	}

	result := make(models.AppList, 0, len(apps))
	for _, app := range apps {
		if appMatchesFilter(IsUndeployed(app), filter) {
			result = append(result, app)
		}
	}
	return result
}

// FilterAppCRs applies optional name search and ListFilter to App CRs before
// pagination/enrichment, so page totals stay correct.
func FilterAppCRs(crs []unstructured.Unstructured, search string, filter ListFilter) []unstructured.Unstructured {
	return filterAppCRs(crs, search, filter)
}

// filterAppCRs applies optional name search and ListFilter to App CRs before
// pagination/enrichment, so page totals stay correct.
func filterAppCRs(crs []unstructured.Unstructured, search string, filter ListFilter) []unstructured.Unstructured {
	if search == "" && filter == ListFilterAll {
		return crs
	}

	lower := strings.ToLower(search)
	matched := make([]unstructured.Unstructured, 0, len(crs))
	for _, cr := range crs {
		if search != "" && !strings.Contains(strings.ToLower(cr.GetName()), lower) {
			continue
		}
		if !appCRMatchesFilter(&cr, filter) {
			continue
		}
		matched = append(matched, cr)
	}
	return matched
}

func appCRMatchesFilter(cr *unstructured.Unstructured, filter ListFilter) bool {
	return appMatchesFilter(IsUndeployedCR(cr), filter)
}

func appMatchesFilter(undeployed bool, filter ListFilter) bool {
	switch filter {
	case ListFilterUndeployed:
		return undeployed
	case ListFilterDeployed:
		return !undeployed
	default:
		return true
	}
}
