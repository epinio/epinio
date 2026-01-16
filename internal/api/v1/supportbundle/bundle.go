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

package supportbundle

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	// SupportBundleNamespace is the namespace where Epinio components are deployed
	SupportBundleNamespace = "epinio"
	// RecentStagingJobsWindow is the time window for collecting recent staging jobs
	RecentStagingJobsWindow = 24 * time.Hour
	// DefaultTailLines is the default number of lines to tail from each component
	DefaultTailLines = int64(1000)
	// MaxTailLines is the maximum number of lines that can be requested
	MaxTailLines = int64(10000)
	// MaxRecommendedBundleSize is the recommended maximum bundle size (500MB)
	// Bundles larger than this will log a warning but won't fail
	MaxRecommendedBundleSize = 500 * 1024 * 1024
	// BundleCollectionTimeout is the maximum time allowed for bundle collection
	BundleCollectionTimeout = 10 * time.Minute
)

// Bundle handles the support bundle download request
// It collects logs from all Epinio components and returns them as a tar archive
func Bundle(c *gin.Context) apierror.APIErrors {
	ctx := c.Request.Context()
	requestID := requestctx.ID(ctx)
	base := helpers.Logger
	if base == nil {
		base = zap.NewNop().Sugar()
	}
	log := base.With("requestId", requestID, "component", "support-bundle")

	log.Infow("starting support bundle collection")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return apierror.InternalError(err)
	}

	// Parse optional query parameters
	// Support both "include_apps" and "include_app_logs" for flexibility
	includeAppLogsParam := c.Query("include_apps")
	if includeAppLogsParam == "" {
		includeAppLogsParam = c.Query("include_app_logs")
	}
	includeAppLogs := includeAppLogsParam == "true"
	tailLinesStr := c.Query("tail")
	tailLines := DefaultTailLines
	if tailLinesStr != "" {
		if parsed, err := parseTailLines(tailLinesStr); err == nil {
			tailLines = parsed
		} else {
			log.Infow("invalid tail parameter, using default", "input", tailLinesStr, "error", err)
		}
	}

	// Add timeout protection - support bundle collection can take time
	// The timeout applies to the entire bundle collection and creation operation
	// If timeout is reached, the context will be cancelled and operations will stop
	ctx, cancel := context.WithTimeout(ctx, BundleCollectionTimeout)
	defer cancel()

	// Create temporary directory for bundle contents
	tmpDir, err := os.MkdirTemp("", "epinio-support-bundle-*")
	if err != nil {
		return apierror.InternalError(errors.Wrap(err, "failed to create temporary directory"))
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Errorw("failed to cleanup temporary directory", "error", err)
		}
	}()

	collector := NewCollector(cluster, tmpDir, tailLines, log)

	// Collect logs from all components
	log.Infow("collecting Epinio server logs")
	if err := collector.CollectEpinioServerLogs(ctx); err != nil {
		log.Errorw("failed to collect Epinio server logs", "error", err)
		// Continue with other components even if one fails
	}

	log.Infow("collecting Epinio UI logs")
	if err := collector.CollectEpinioUILogs(ctx); err != nil {
		log.Errorw("failed to collect Epinio UI logs", "error", err)
	}

	log.Infow("collecting staging job logs")
	if err := collector.CollectStagingJobLogs(ctx); err != nil {
		log.Errorw("failed to collect staging job logs", "error", err)
	}

	log.Infow("collecting Minio logs")
	if err := collector.CollectMinioLogs(ctx); err != nil {
		log.Errorw("failed to collect Minio logs", "error", err)
	}

	log.Infow("collecting container registry logs")
	if err := collector.CollectRegistryLogs(ctx); err != nil {
		log.Errorw("failed to collect container registry logs", "error", err)
	}

	// Optionally collect application logs
	if includeAppLogs {
		log.Infow("collecting application logs", "include_apps_param", c.Query("include_apps"), "include_app_logs_param", c.Query("include_app_logs"))
		if err := collector.CollectApplicationLogs(ctx); err != nil {
			log.Errorw("failed to collect application logs", "error", err)
		} else {
			log.Infow("application logs collection completed")
		}
	} else {
		log.Debugw("skipping application logs collection (include_apps or include_app_logs not set to true)")
	}

	// Check if any logs were collected before creating archive
	hasLogs, err := hasCollectedLogs(tmpDir)
	if err != nil {
		return apierror.InternalError(errors.Wrap(err, "failed to check collected logs"))
	}
	if !hasLogs {
		return apierror.NewAPIError("no logs could be collected from any Epinio components", http.StatusInternalServerError)
	}

	// Create tar archive
	log.Infow("creating support bundle archive")
	archivePath, err := collector.CreateArchive(ctx)
	if err != nil {
		return apierror.InternalError(errors.Wrap(err, "failed to create support bundle archive"))
	}
	defer func() {
		if err := os.Remove(archivePath); err != nil {
			log.Errorw("failed to cleanup archive file", "error", err)
		}
	}()

	// Get file info for content length
	fileInfo, err := os.Stat(archivePath)
	if err != nil {
		return apierror.InternalError(errors.Wrap(err, "failed to get archive file info"))
	}

	// Validate archive is not empty (additional safety check after creation)
	if fileInfo.Size() == 0 {
		return apierror.NewAPIError("support bundle archive is empty - no logs were included", http.StatusInternalServerError)
	}

	// Warn if archive is very large - log but don't fail
	if fileInfo.Size() > MaxRecommendedBundleSize {
		sizeMB := fileInfo.Size() / (1024 * 1024)
		log.Infow("support bundle is very large, may cause download issues", "size_mb", sizeMB, "max_recommended_mb", MaxRecommendedBundleSize/(1024*1024))
	}

	// Generate filename with timestamp
	filename := fmt.Sprintf("epinio-support-bundle-%s.tar.gz", time.Now().Format("20060102-150405"))

	// Open the file for streaming
	file, err := os.Open(archivePath)
	if err != nil {
		return apierror.InternalError(errors.Wrap(err, "failed to open archive file"))
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Errorw("failed to close archive file", "error", err)
		}
	}()

	// Set headers for file download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/gzip")
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Stream the file to the client using DataFromReader (similar to part.go)
	c.DataFromReader(http.StatusOK, fileInfo.Size(), "application/gzip", bufio.NewReader(file), nil)

	log.Infow("support bundle download completed successfully", "filename", filename, "size_bytes", fileInfo.Size(), "size_mb", fileInfo.Size()/(1024*1024))

	return nil
}

// parseTailLines parses and validates the tail query parameter
// Returns an error if the value is invalid or out of range
func parseTailLines(tailStr string) (int64, error) {
	var tail int64
	if _, err := fmt.Sscanf(tailStr, "%d", &tail); err != nil {
		return 0, errors.Wrapf(err, "invalid tail parameter: %q (must be a number)", tailStr)
	}
	if tail < 0 {
		return 0, fmt.Errorf("tail parameter must be non-negative, got: %d", tail)
	}
	if tail > MaxTailLines {
		return 0, fmt.Errorf("tail parameter cannot exceed %d, got: %d", MaxTailLines, tail)
	}
	return tail, nil
}

// errStopWalk is a sentinel error to stop the filepath.Walk early
var errStopWalk = errors.New("stop walk")

// hasCollectedLogs checks if any log files were collected in the bundle directory
func hasCollectedLogs(bundleDir string) (bool, error) {
	hasFiles := false
	err := filepath.Walk(bundleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if it's a log file (not a directory and not the archive file itself)
		// Use case-insensitive check for file extension
		ext := strings.ToLower(filepath.Ext(path))
		if !info.IsDir() && ext == ".log" {
			hasFiles = true
			// Stop the entire walk immediately
			return errStopWalk
		}
		return nil
	})
	// Check if we stopped early due to finding a log file
	if err == errStopWalk {
		return true, nil
	}
	return hasFiles, err
}

