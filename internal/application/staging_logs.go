// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sync"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/helmchart"
	"github.com/epinio/epinio/internal/s3manager"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	// stagingLogsPrefix is the key prefix of all persisted staging logs in S3.
	stagingLogsPrefix = "staging-logs"

	// defaultStagingLogsMaxSizeMB is used when the configured size cap is not positive.
	defaultStagingLogsMaxSizeMB = 10

	// stagingLogsContentType is the content type of the stored log objects.
	stagingLogsContentType = "application/x-ndjson"
)

// StagingLogsSettings are the stagingWorkloads helm values controlling the
// persistence of staging logs.
type StagingLogsSettings struct {
	Enabled  bool
	MaxBytes int64
}

// NewStagingLogsSettings converts the helm values into StagingLogsSettings.
func NewStagingLogsSettings(enabled bool, maxSizeMB int) StagingLogsSettings {
	if maxSizeMB <= 0 {
		maxSizeMB = defaultStagingLogsMaxSizeMB
	}
	return StagingLogsSettings{
		Enabled:  enabled,
		MaxBytes: int64(maxSizeMB) * 1024 * 1024,
	}
}

// stagingLogObjectKey is the key of the logs of one staging run.
func stagingLogObjectKey(namespace, stageID string) string {
	return fmt.Sprintf("%s/%s/%s.ndjson", stagingLogsPrefix, namespace, stageID)
}

// stagingLogPointerKey is the key of the object naming the stage whose logs
// are currently stored for an app. It lets the logs of the previous run be
// found and removed, and the app's logs be found on app deletion.
func stagingLogPointerKey(namespace, appName string) string {
	return fmt.Sprintf("%s/%s/apps/%s", stagingLogsPrefix, namespace, appName)
}

// encodeStagingLogs serializes the log lines as newline-delimited JSON, the same
// format the logs endpoint sends over the websocket. When the lines do not fit
// into maxBytes the oldest lines are dropped and a notice is prepended.
func encodeStagingLogs(lines []tailer.ContainerLogLine, maxBytes int64) ([]byte, error) {
	encoded := make([][]byte, len(lines))
	for i, line := range lines {
		data, err := json.Marshal(line)
		if err != nil {
			return nil, errors.Wrap(err, "encoding a staging log line")
		}
		encoded[i] = data
	}

	// Walk backwards so the most recent lines are the ones that are kept.
	var size int64
	start := len(encoded)
	for i := len(encoded) - 1; i >= 0; i-- {
		lineSize := int64(len(encoded[i]) + 1)
		if size+lineSize > maxBytes {
			break
		}
		size += lineSize
		start = i
	}

	var buf bytes.Buffer
	if start > 0 {
		notice, err := json.Marshal(tailer.ContainerLogLine{
			Message: fmt.Sprintf("... %d earlier log lines dropped, the stored logs are limited to %d bytes ...",
				start, maxBytes),
		})
		if err != nil {
			return nil, errors.Wrap(err, "encoding the truncation notice")
		}
		buf.Write(notice)
		buf.WriteByte('\n')
	}
	for _, data := range encoded[start:] {
		buf.Write(data)
		buf.WriteByte('\n')
	}

	return buf.Bytes(), nil
}

// collectStagingLogs reads the logs of all containers of the staging run stageID.
func collectStagingLogs(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	namespace, stageID string,
) ([]tailer.ContainerLogLine, error) {
	selector := labels.Set{
		"app.kubernetes.io/component": "staging",
		"app.kubernetes.io/part-of":   namespace,
		models.EpinioStageIDLabel:     stageID,
	}.AsSelector()

	config := &tailer.Config{
		Namespace:      helmchart.Namespace(),
		ContainerQuery: regexp.MustCompile(".*"),
		Timestamps:     true,
		LabelSelector:  selector,
		Ordered:        true,
	}

	logChan := make(chan tailer.ContainerLogLine)
	collected := make(chan []tailer.ContainerLogLine, 1)
	go func() {
		var lines []tailer.ContainerLogLine
		for line := range logChan {
			lines = append(lines, line)
		}
		collected <- lines
	}()

	err := tailer.FetchLogs(ctx, logChan, &sync.WaitGroup{}, config, cluster)
	close(logChan)
	lines := <-collected
	if err != nil {
		return nil, errors.Wrap(err, "reading the staging logs")
	}

	return lines, nil
}

// stagingLogsS3 connects to the S3 storage the blobs are kept in.
func stagingLogsS3(ctx context.Context, cluster *kubernetes.Cluster) (*s3manager.Manager, error) {
	connectionDetails, err := s3manager.GetConnectionDetails(
		ctx, cluster, helmchart.Namespace(), helmchart.S3ConnectionDetailsSecretName)
	if err != nil {
		return nil, errors.Wrap(err, "fetching the S3 connection details")
	}

	s3m, err := s3manager.New(connectionDetails)
	if err != nil {
		return nil, errors.Wrap(err, "creating an S3 manager")
	}

	return s3m, nil
}

// readLatestStageID returns the stage whose logs are stored for the app, or an
// empty string when there are none.
func readLatestStageID(ctx context.Context, s3m *s3manager.Manager, namespace, appName string) (string, error) {
	body, _, _, err := s3m.Open(ctx, stagingLogPointerKey(namespace, appName))
	if err != nil {
		if s3manager.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}
	defer func() {
		_ = body.Close()
	}()

	data, err := io.ReadAll(body)
	if err != nil {
		return "", errors.Wrap(err, "reading the staging logs pointer")
	}

	return string(data), nil
}

// PersistStagingLogs stores the logs of the finished staging run stageID as the
// latest staging logs of the app and removes the logs of the previous run. It
// does nothing when persisting is disabled or the app has been deleted in the
// meantime.
func PersistStagingLogs(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	settings StagingLogsSettings,
	namespace, appName, stageID string,
) error {
	if !settings.Enabled {
		return nil
	}

	app, err := Lookup(ctx, cluster, namespace, appName)
	if err != nil {
		return errors.Wrap(err, "looking up the app")
	}
	if app == nil {
		return nil
	}

	lines, err := collectStagingLogs(ctx, cluster, namespace, stageID)
	if err != nil {
		return err
	}

	body, err := encodeStagingLogs(lines, settings.MaxBytes)
	if err != nil {
		return err
	}

	s3m, err := stagingLogsS3(ctx, cluster)
	if err != nil {
		return err
	}

	previousStageID, err := readLatestStageID(ctx, s3m, namespace, appName)
	if err != nil {
		return errors.Wrap(err, "reading the previous staging logs")
	}

	if err := s3m.PutObject(ctx, stagingLogObjectKey(namespace, stageID), body, stagingLogsContentType); err != nil {
		return errors.Wrap(err, "storing the staging logs")
	}

	if err := s3m.PutObject(ctx, stagingLogPointerKey(namespace, appName), []byte(stageID), "text/plain"); err != nil {
		return errors.Wrap(err, "updating the staging logs pointer")
	}

	if previousStageID != "" && previousStageID != stageID {
		if err := s3m.DeleteObject(ctx, stagingLogObjectKey(namespace, previousStageID)); err != nil {
			return errors.Wrap(err, "removing the previous staging logs")
		}
	}

	return nil
}

// OpenStagingLogs returns the stored logs of the staging run stageID as
// newline-delimited JSON. found is false when no logs are stored for that stage,
// for example because a newer run replaced them or they were never persisted.
func OpenStagingLogs(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	namespace, stageID string,
) (body io.ReadCloser, found bool, err error) {
	s3m, err := stagingLogsS3(ctx, cluster)
	if err != nil {
		return nil, false, err
	}

	body, _, _, err = s3m.Open(ctx, stagingLogObjectKey(namespace, stageID))
	if err != nil {
		if s3manager.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, errors.Wrap(err, "opening the staging logs")
	}

	return body, true, nil
}

// DeleteStagingLogs removes the stored staging logs of an app, together with
// the pointer to them. Missing logs are not an error.
func DeleteStagingLogs(ctx context.Context, cluster *kubernetes.Cluster, namespace, appName string) error {
	s3m, err := stagingLogsS3(ctx, cluster)
	if err != nil {
		return err
	}

	stageID, err := readLatestStageID(ctx, s3m, namespace, appName)
	if err != nil {
		return errors.Wrap(err, "reading the staging logs pointer")
	}

	if stageID != "" {
		if err := s3m.DeleteObject(ctx, stagingLogObjectKey(namespace, stageID)); err != nil {
			return errors.Wrap(err, "removing the staging logs")
		}
	}

	if err := s3m.DeleteObject(ctx, stagingLogPointerKey(namespace, appName)); err != nil {
		return errors.Wrap(err, "removing the staging logs pointer")
	}

	return nil
}
