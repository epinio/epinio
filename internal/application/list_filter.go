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
// the app's buildstatus (see models.AppBuildStatusBuild / Deployed).
type ListFilter int

const (
	// ListFilterAll returns every application CR, regardless of buildstatus.
	ListFilterAll ListFilter = iota
	// ListFilterDeployed excludes apps whose last completed action was a
	// build-only stage (buildstatus == "build"). Used by the regular
	// applications list endpoints so the UI "Apps" view stays clean.
	ListFilterDeployed
	// ListFilterUndeployed returns only build-only apps (buildstatus ==
	// "build"). Used by the undeployed/builds list endpoints.
	ListFilterUndeployed
)

// IsUndeployed reports whether the app is build-only (staged, not deployed).
func IsUndeployed(app models.App) bool {
	return app.BuildStatus == models.AppBuildStatusBuild
}

// IsUndeployedCR reports whether the App CR is build-only, based on
// spec.buildstatus. Missing or unreadable buildstatus is treated as not
// undeployed (same as an app that was never build-only staged).
func IsUndeployedCR(app *unstructured.Unstructured) bool {
	status, err := BuildStatus(app)
	if err != nil {
		return false
	}
	return status == models.AppBuildStatusBuild
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
