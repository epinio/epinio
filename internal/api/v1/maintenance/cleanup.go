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

package maintenance

import (
	"context"
	"strconv"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gin-gonic/gin"
)

// CleanupStaleCachesRequest represents the request body for cleanup endpoint
type CleanupStaleCachesRequest struct {
	StaleDays      *int  `json:"staleDays,omitempty"`      // Number of days after which cache is considered stale (default: 30)
	CheckAppExists *bool `json:"checkAppExists,omitempty"` // If true, only delete caches for apps that no longer exist (default: true for safety)
	DryRun         *bool `json:"dryRun,omitempty"`         // If true, only report what would be deleted without actually deleting (default: false)
}

// CleanupStaleCachesResponse represents the response from cleanup endpoint
type CleanupStaleCachesResponse struct {
	DeletedCount int                     `json:"deletedCount"`
	StaleCaches  []models.StaleCacheInfo `json:"staleCaches"`
	Errors       []string                `json:"errors,omitempty"`
	DryRun       bool                    `json:"dryRun"`
}

// performCleanup is a helper function that performs the actual cleanup logic
func performCleanup(ctx context.Context, cluster *kubernetes.Cluster, req CleanupStaleCachesRequest) (int, []application.StaleCacheInfo, []error, apierror.APIErrors) {
	logger := requestctx.Logger(ctx)

	staleDays := application.DefaultStaleCacheDays
	if req.StaleDays != nil && *req.StaleDays > 0 {
		staleDays = *req.StaleDays
	}

	checkAppExists := true
	if req.CheckAppExists != nil {
		checkAppExists = *req.CheckAppExists
	}

	dryRun := false
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}

	// Find stale caches
	staleCaches, err := application.FindStaleCachePVCs(ctx, cluster, staleDays)
	if err != nil {
		return 0, nil, nil, apierror.InternalError(err, "finding stale cache PVCs")
	}

	// Filter by app existence if requested
	if checkAppExists {
		var filteredCaches []application.StaleCacheInfo
		for _, cacheInfo := range staleCaches {
			appRef := models.NewAppRef(cacheInfo.AppName, cacheInfo.AppNamespace)
			exists, err := application.AppExists(ctx, cluster, appRef)
			if err != nil {
				// If we can't check, be conservative and skip
				logger.V(1).Info("skipping cache check due to error",
					"pvc", cacheInfo.PVC.Name,
					"error", err.Error(),
				)
				continue
			}
			if !exists {
				filteredCaches = append(filteredCaches, cacheInfo)
			}
		}
		staleCaches = filteredCaches
	}

	// If dry-run, don't actually delete
	if dryRun {
		return 0, staleCaches, nil, nil
	}

	// Delete the stale caches
	deletedCount, deleteErrors := application.DeleteStaleCachePVCs(ctx, cluster, staleCaches)

	return deletedCount, staleCaches, deleteErrors, nil
}

// CleanupStaleCaches handles the API endpoint POST /api/v1/maintenance/cleanup-stale-caches
// It identifies and optionally deletes stale cache PVCs that haven't been used for more than staleDays.
// The endpoint supports dry-run mode for testing and verification.
func CleanupStaleCaches(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Parse request body (optional)
	var req CleanupStaleCachesRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			return apierror.NewBadRequestError(err.Error())
		}
	}

	// Set defaults - use pointers to distinguish between omitted and zero values
	staleDays := application.DefaultStaleCacheDays
	if req.StaleDays != nil && *req.StaleDays > 0 {
		staleDays = *req.StaleDays
	}

	// Default checkAppExists to true for safety (only delete caches for deleted apps)
	// This prevents accidentally deleting caches for active apps that just haven't built recently
	checkAppExists := true
	if req.CheckAppExists != nil {
		checkAppExists = *req.CheckAppExists
	}

	dryRun := false
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}

	logger.Info("cleanup stale caches",
		"staleDays", staleDays,
		"checkAppExists", checkAppExists,
		"dryRun", dryRun,
	)

	// Perform cleanup
	cleanupReq := CleanupStaleCachesRequest{
		StaleDays:      &staleDays,
		CheckAppExists: &checkAppExists,
		DryRun:         &dryRun,
	}
	deletedCount, staleCaches, cleanupErrors, apiErr := performCleanup(ctx, cluster, cleanupReq)
	if apiErr != nil {
		return apiErr
	}

	// Convert errors to strings
	errorStrings := make([]string, len(cleanupErrors))
	for i, err := range cleanupErrors {
		errorStrings[i] = err.Error()
	}

	// Convert StaleCacheInfo to models
	staleCacheModels := make([]models.StaleCacheInfo, len(staleCaches))
	for i, cache := range staleCaches {
		staleCacheModels[i] = models.StaleCacheInfo{
			PVCName:        cache.PVC.Name,
			AppName:        cache.AppName,
			AppNamespace:   cache.AppNamespace,
			LastBuildTime:  cache.LastBuildTime,
			DaysSinceBuild: cache.DaysSinceBuild,
		}
	}

	responseData := CleanupStaleCachesResponse{
		DeletedCount: deletedCount,
		StaleCaches:  staleCacheModels,
		Errors:       errorStrings,
		DryRun:       dryRun,
	}

	// In dry-run mode, set deletedCount to the number of caches that would be deleted
	if dryRun {
		responseData.DeletedCount = len(staleCaches)
		logger.Info("dry-run cleanup",
			"wouldDelete", len(staleCaches),
		)
	} else {
		logger.Info("cleanup completed",
			"deleted", deletedCount,
			"errors", len(cleanupErrors),
		)
	}

	response.OKReturn(c, responseData)
	return nil
}

// CleanupStaleCachesQuery handles GET /api/v1/maintenance/cleanup-stale-caches?staleDays=30&dryRun=true
// This allows calling the cleanup via simple GET requests (useful for cron jobs)
func CleanupStaleCachesQuery(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	logger := requestctx.Logger(ctx)

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Parse query parameters
	staleDays := application.DefaultStaleCacheDays
	if staleDaysStr := c.Query("staleDays"); staleDaysStr != "" {
		parsed, err := strconv.Atoi(staleDaysStr)
		if err != nil {
			return apierror.NewBadRequestError("invalid staleDays parameter: " + err.Error())
		}
		if parsed > 0 {
			staleDays = parsed
		}
	}

	// For GET requests, default checkAppExists to true for safety; allow explicit false
	checkAppExists := c.Query("checkAppExists") != "false"

	dryRun := c.Query("dryRun") == "true"

	cleanupReq := CleanupStaleCachesRequest{
		StaleDays:      &staleDays,
		CheckAppExists: &checkAppExists,
		DryRun:         &dryRun,
	}

	logger.Info("cleanup stale caches (query)",
		"staleDays", staleDays,
		"checkAppExists", checkAppExists,
		"dryRun", dryRun,
	)

	// Perform cleanup
	deletedCount, staleCaches, cleanupErrors, apiErr := performCleanup(ctx, cluster, cleanupReq)
	if apiErr != nil {
		return apiErr
	}

	// Convert errors to strings
	errorStrings := make([]string, len(cleanupErrors))
	for i, err := range cleanupErrors {
		errorStrings[i] = err.Error()
	}

	// Convert StaleCacheInfo to models
	staleCacheModels := make([]models.StaleCacheInfo, len(staleCaches))
	for i, cache := range staleCaches {
		staleCacheModels[i] = models.StaleCacheInfo{
			PVCName:        cache.PVC.Name,
			AppName:        cache.AppName,
			AppNamespace:   cache.AppNamespace,
			LastBuildTime:  cache.LastBuildTime,
			DaysSinceBuild: cache.DaysSinceBuild,
		}
	}

	responseData := CleanupStaleCachesResponse{
		DeletedCount: deletedCount,
		StaleCaches:  staleCacheModels,
		Errors:       errorStrings,
		DryRun:       dryRun,
	}

	// In dry-run mode, set deletedCount to the number of caches that would be deleted
	if dryRun {
		responseData.DeletedCount = len(staleCaches)
		logger.Info("dry-run cleanup (query)",
			"wouldDelete", len(staleCaches),
		)
	} else {
		logger.Info("cleanup completed (query)",
			"deleted", deletedCount,
			"errors", len(cleanupErrors),
		)
	}

	response.OKReturn(c, responseData)
	return nil
}
