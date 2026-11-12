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
	"sort"
	"strings"

	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// NeedsEnrichedAppSort reports whether sorting by field requires fully
// enriched App models (workload/staging status) rather than App CRs alone.
func NeedsEnrichedAppSort(field string) bool {
	switch field {
	case "status", "deploymentStatus":
		return true
	default:
		return false
	}
}

// SortAppCRs sorts App CRs in place by the given field before pagination.
// Supported: name, createdAt, namespace, buildStatus, builtImageUrl, username.
// Unknown fields and enrichment-only fields are a no-op.
func SortAppCRs(crs []unstructured.Unstructured, field string, descending bool) {
	if field == "" || len(crs) < 2 {
		return
	}

	less := appCRLess(field)
	if less == nil {
		return
	}

	sort.SliceStable(crs, func(i, j int) bool {
		if descending {
			return less(crs[j], crs[i])
		}
		return less(crs[i], crs[j])
	})
}

// SortApps sorts enriched App models in place by the given field.
// Supported: name, createdAt, namespace, buildStatus, builtImageUrl, username,
// status, deploymentStatus. Unknown fields are a no-op.
func SortApps(apps models.AppList, field string, descending bool) {
	if field == "" || len(apps) < 2 {
		return
	}

	less := appLess(field)
	if less == nil {
		return
	}

	sort.SliceStable(apps, func(i, j int) bool {
		if descending {
			return less(apps[j], apps[i])
		}
		return less(apps[i], apps[j])
	})
}

func appCRLess(field string) func(a, b unstructured.Unstructured) bool {
	switch field {
	case "name":
		return func(a, b unstructured.Unstructured) bool {
			return strings.ToLower(a.GetName()) < strings.ToLower(b.GetName())
		}
	case "namespace":
		return func(a, b unstructured.Unstructured) bool {
			an, bn := a.GetNamespace(), b.GetNamespace()
			if an == bn {
				return strings.ToLower(a.GetName()) < strings.ToLower(b.GetName())
			}
			return strings.ToLower(an) < strings.ToLower(bn)
		}
	case "createdAt":
		return func(a, b unstructured.Unstructured) bool {
			at, bt := a.GetCreationTimestamp().Time, b.GetCreationTimestamp().Time
			if at.Equal(bt) {
				return strings.ToLower(a.GetName()) < strings.ToLower(b.GetName())
			}
			return at.Before(bt)
		}
	case "buildStatus":
		return func(a, b unstructured.Unstructured) bool {
			return compareWithName(
				appCRNestedString(a, "spec", "buildstatus"),
				appCRNestedString(b, "spec", "buildstatus"),
				a.GetName(), b.GetName(),
			)
		}
	case "builtImageUrl":
		return func(a, b unstructured.Unstructured) bool {
			return compareWithName(
				appCRNestedString(a, "spec", "builtimageurl"),
				appCRNestedString(b, "spec", "builtimageurl"),
				a.GetName(), b.GetName(),
			)
		}
	case "username":
		return func(a, b unstructured.Unstructured) bool {
			return compareWithName(
				appCRAnnotation(a, models.EpinioCreatedByAnnotation),
				appCRAnnotation(b, models.EpinioCreatedByAnnotation),
				a.GetName(), b.GetName(),
			)
		}
	default:
		return nil
	}
}

func appLess(field string) func(a, b models.App) bool {
	switch field {
	case "name":
		return func(a, b models.App) bool {
			return strings.ToLower(a.Meta.Name) < strings.ToLower(b.Meta.Name)
		}
	case "namespace":
		return func(a, b models.App) bool {
			an, bn := a.Meta.Namespace, b.Meta.Namespace
			if an == bn {
				return strings.ToLower(a.Meta.Name) < strings.ToLower(b.Meta.Name)
			}
			return strings.ToLower(an) < strings.ToLower(bn)
		}
	case "createdAt":
		return func(a, b models.App) bool {
			at, bt := a.Meta.CreatedAt.Time, b.Meta.CreatedAt.Time
			if at.Equal(bt) {
				return strings.ToLower(a.Meta.Name) < strings.ToLower(b.Meta.Name)
			}
			return at.Before(bt)
		}
	case "buildStatus":
		return func(a, b models.App) bool {
			return compareWithName(a.BuildStatus, b.BuildStatus, a.Meta.Name, b.Meta.Name)
		}
	case "builtImageUrl":
		return func(a, b models.App) bool {
			return compareWithName(a.BuiltImageURL, b.BuiltImageURL, a.Meta.Name, b.Meta.Name)
		}
	case "username":
		return func(a, b models.App) bool {
			return compareWithName(appUsername(a), appUsername(b), a.Meta.Name, b.Meta.Name)
		}
	case "status":
		return func(a, b models.App) bool {
			return compareWithName(string(a.Status), string(b.Status), a.Meta.Name, b.Meta.Name)
		}
	case "deploymentStatus":
		return func(a, b models.App) bool {
			return compareWithName(appDeploymentStatus(a), appDeploymentStatus(b), a.Meta.Name, b.Meta.Name)
		}
	default:
		return nil
	}
}

func compareWithName(aVal, bVal, aName, bName string) bool {
	al, bl := strings.ToLower(aVal), strings.ToLower(bVal)
	if al == bl {
		return strings.ToLower(aName) < strings.ToLower(bName)
	}
	return al < bl
}

func appCRNestedString(cr unstructured.Unstructured, fields ...string) string {
	value, _, _ := unstructured.NestedString(cr.UnstructuredContent(), fields...)
	return value
}

func appCRAnnotation(cr unstructured.Unstructured, key string) string {
	annotations := cr.GetAnnotations()
	if annotations == nil {
		return ""
	}
	return annotations[key]
}

func appUsername(app models.App) string {
	if app.Workload != nil {
		return app.Workload.Username
	}
	return ""
}

func appDeploymentStatus(app models.App) string {
	if app.Workload != nil {
		return app.Workload.Status
	}
	return ""
}
