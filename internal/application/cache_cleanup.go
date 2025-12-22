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
	"context"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	apibatchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	// DefaultStaleCacheDays is the default number of days after which a cache is considered stale
	DefaultStaleCacheDays = 30
	// CacheTypeLabel is the label key used to identify cache PVCs
	CacheTypeLabel = "epinio.io/type"
	// CacheTypeValue is the label value for cache PVCs
	CacheTypeValue = "cache"
)

// StaleCacheInfo contains information about a stale cache PVC
type StaleCacheInfo struct {
	PVC            corev1.PersistentVolumeClaim
	AppName        string
	AppNamespace   string
	LastBuildTime  time.Time
	DaysSinceBuild int
}

// Export StaleCacheInfo for use in API layer

// JobListerForCacheCleanup is an interface for listing jobs, used for testing
type JobListerForCacheCleanup interface {
	ListJobs(ctx context.Context, namespace, selector string) (*apibatchv1.JobList, error)
}

// GetLastBuildTime returns the most recent build time for an application.
// It searches for the latest staging job completion time. If no jobs are found,
// it returns the PVC creation time as a fallback.
func GetLastBuildTime(ctx context.Context, jobLister JobListerForCacheCleanup, appRef models.AppRef) (time.Time, error) {
	// Find all staging jobs for this app
	selector := labels.Set(map[string]string{
		"app.kubernetes.io/component": "staging",
		"app.kubernetes.io/name":      appRef.Name,
		"app.kubernetes.io/part-of":   appRef.Namespace,
	}).String()

	jobList, err := jobLister.ListJobs(ctx, helmchart.Namespace(), selector)
	if err != nil {
		return time.Time{}, errors.Wrap(err, "listing staging jobs")
	}

	var latestCompletionTime *metav1.Time

	// Find the most recent job completion time
	for _, job := range jobList.Items {
		if job.Status.CompletionTime != nil {
			if latestCompletionTime == nil || job.Status.CompletionTime.After(latestCompletionTime.Time) {
				latestCompletionTime = job.Status.CompletionTime
			}
		}
	}

	if latestCompletionTime != nil {
		return latestCompletionTime.Time, nil
	}

	// If no completed jobs found, return zero time (will use PVC creation time as fallback)
	return time.Time{}, nil
}

// FindStaleCachePVCs identifies cache PVCs that are stale (unused for more than staleDays).
// It returns a list of StaleCacheInfo containing the PVC and relevant metadata.
func FindStaleCachePVCs(ctx context.Context, cluster *kubernetes.Cluster, staleDays int) ([]StaleCacheInfo, error) {
	if staleDays <= 0 {
		staleDays = DefaultStaleCacheDays
	}

	// List cache PVCs - first try labeled ones, then also check unlabeled ones for backward compatibility
	selector := labels.Set(map[string]string{
		CacheTypeLabel: CacheTypeValue,
	}).String()

	pvcList, err := cluster.Kubectl.CoreV1().PersistentVolumeClaims(helmchart.Namespace()).
		List(ctx, metav1.ListOptions{
			LabelSelector: selector,
		})
	if err != nil {
		return nil, errors.Wrap(err, "listing cache PVCs")
	}

	// Also list unlabeled PVCs that might be old cache PVCs (for backward compatibility)
	// We identify them by name pattern: contains "-cache-"
	allPVCs, err := cluster.Kubectl.CoreV1().PersistentVolumeClaims(helmchart.Namespace()).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "listing all PVCs")
	}

	// Add unlabeled PVCs that match cache naming pattern
	labeledPVCs := make(map[string]bool)
	for _, pvc := range pvcList.Items {
		labeledPVCs[pvc.Name] = true
	}

	for _, pvc := range allPVCs.Items {
		// Skip if already in labeled list
		if labeledPVCs[pvc.Name] {
			continue
		}
		// Check if name contains "-cache-" pattern (old cache PVCs)
		if strings.Contains(pvc.Name, "-cache-") {
			pvcList.Items = append(pvcList.Items, pvc)
		}
	}

	var staleCaches []StaleCacheInfo
	now := time.Now()
	staleThreshold := time.Duration(staleDays) * 24 * time.Hour

	for _, pvc := range pvcList.Items {
		// Extract app information from labels
		appName := pvc.Labels["app.kubernetes.io/name"]
		appNamespace := pvc.Labels["app.kubernetes.io/part-of"]

		// If labels are missing, try to parse from PVC name (for backward compatibility)
		if appName == "" || appNamespace == "" {
			parsedName, parsedNamespace := ParseCachePVCName(pvc.Name)
			if parsedName == "" || parsedNamespace == "" {
				// Skip PVCs we can't identify
				continue
			}
			appName = parsedName
			appNamespace = parsedNamespace
		}

		appRef := models.NewAppRef(appName, appNamespace)

		// Get the last build time for this app
		lastBuildTime, err := GetLastBuildTime(ctx, cluster, appRef)
		if err != nil {
			// Log error but continue with other PVCs
			// Use PVC creation time as fallback
			lastBuildTime = pvc.CreationTimestamp.Time
		}

		// If no build time found, use PVC creation time
		if lastBuildTime.IsZero() {
			lastBuildTime = pvc.CreationTimestamp.Time
		}

		// Calculate days since last build
		daysSinceBuild := int(now.Sub(lastBuildTime).Hours() / 24)

		// Check if cache is stale
		if now.Sub(lastBuildTime) > staleThreshold {
			staleCaches = append(staleCaches, StaleCacheInfo{
				PVC:            pvc,
				AppName:        appName,
				AppNamespace:   appNamespace,
				LastBuildTime:  lastBuildTime,
				DaysSinceBuild: daysSinceBuild,
			})
		}
	}

	return staleCaches, nil
}

// ParseCachePVCName attempts to extract app name and namespace from a cache PVC name.
// Cache PVC names are generated using names.GenerateResourceName(namespace, "cache", appName)
// which creates: {sanitized-prefix}-{40-char-hash}
// where prefix is DNSLabelSafe(namespace + "-cache-" + appName) truncated to fit.
// This is best-effort parsing for backward compatibility with old PVCs.
// Returns empty strings if parsing fails.
// Exported for testing purposes.
func ParseCachePVCName(pvcName string) (appName, appNamespace string) {
	// Look for "-cache-" in the name
	cacheIdx := -1
	for i := 0; i <= len(pvcName)-7; i++ {
		if i+7 <= len(pvcName) && pvcName[i:i+7] == "-cache-" {
			cacheIdx = i
			break
		}
	}

	if cacheIdx == -1 || cacheIdx == 0 {
		return "", ""
	}

	// Extract namespace (everything before "-cache-")
	appNamespace = pvcName[:cacheIdx]

	// Extract everything after "-cache-"
	afterCache := pvcName[cacheIdx+7:]

	// The name ends with a 40-character SHA1 hash
	// Everything between "-cache-" and the hash is the app name (possibly truncated/sanitized)
	if len(afterCache) <= 41 {
		// Too short to have both app name and hash
		return "", ""
	}

	// Assume last 40 chars are the hash, everything before that (minus trailing dash) is app name
	hashStart := len(afterCache) - 40
	if hashStart <= 0 {
		return "", ""
	}

	appName = afterCache[:hashStart]
	// Remove trailing dash if present
	if len(appName) > 0 && appName[len(appName)-1] == '-' {
		appName = appName[:len(appName)-1]
	}

	// Basic validation
	if appNamespace == "" || appName == "" {
		return "", ""
	}

	return appName, appNamespace
}

// DeleteStaleCachePVCs deletes the provided list of stale cache PVCs.
// It returns the number of successfully deleted PVCs and any errors encountered.
func DeleteStaleCachePVCs(ctx context.Context, cluster *kubernetes.Cluster, staleCaches []StaleCacheInfo) (int, []error) {
	var deletedCount int
	var deleteErrors []error

	for _, cacheInfo := range staleCaches {
		err := cluster.Kubectl.CoreV1().PersistentVolumeClaims(helmchart.Namespace()).
			Delete(ctx, cacheInfo.PVC.Name, metav1.DeleteOptions{})
		if err != nil {
			deleteErrors = append(deleteErrors, errors.Wrapf(err, "deleting cache PVC %s for app %s/%s",
				cacheInfo.PVC.Name, cacheInfo.AppNamespace, cacheInfo.AppName))
		} else {
			deletedCount++
		}
	}

	return deletedCount, deleteErrors
}

// CleanupStaleCaches performs the complete cleanup process:
// 1. Finds all stale cache PVCs
// 2. Optionally checks if the app still exists (if checkAppExists is true)
// 3. Deletes the stale PVCs
// Returns the number of deleted PVCs and any errors
func CleanupStaleCaches(ctx context.Context, cluster *kubernetes.Cluster, staleDays int, checkAppExists bool) (int, []StaleCacheInfo, []error) {
	// Find stale caches
	staleCaches, err := FindStaleCachePVCs(ctx, cluster, staleDays)
	if err != nil {
		return 0, nil, []error{err}
	}

	// If checkAppExists is true, filter out PVCs for apps that still exist
	if checkAppExists {
		var filteredCaches []StaleCacheInfo
		for _, cacheInfo := range staleCaches {
			appRef := models.NewAppRef(cacheInfo.AppName, cacheInfo.AppNamespace)
			exists, err := AppExists(ctx, cluster, appRef)
			if err != nil {
				// If we can't check, be conservative and skip deletion
				continue
			}
			if !exists {
				filteredCaches = append(filteredCaches, cacheInfo)
			}
		}
		staleCaches = filteredCaches
	}

	// Delete the stale caches
	deletedCount, deleteErrors := DeleteStaleCachePVCs(ctx, cluster, staleCaches)

	return deletedCount, staleCaches, deleteErrors
}

// AppExists checks if an application still exists in the cluster
func AppExists(ctx context.Context, cluster *kubernetes.Cluster, appRef models.AppRef) (bool, error) {
	client, err := cluster.ClientApp()
	if err != nil {
		return false, errors.Wrap(err, "getting app client")
	}

	_, err = client.Namespace(appRef.Namespace).Get(ctx, appRef.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrap(err, "checking if app exists")
	}

	return true, nil
}
