// Copyright © 2021 - 2026 KRUMWARE LLC
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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/duration"
	"github.com/epinio/epinio/internal/helmchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// stagingLogsSettings reads the persistLogs helm values from the staging
// configuration. Persisting stays disabled when the configuration cannot be read.
func stagingLogsSettings(ctx context.Context, cluster *kubernetes.Cluster) application.StagingLogsSettings {
	log := requestctx.Logger(ctx)

	configmap, err := cluster.GetConfigMap(ctx, helmchart.Namespace(), "epinio-stage-scripts")
	if err != nil {
		log.Errorw("failed to read the staging configuration, staging logs are not persisted", "error", err)
		return application.StagingLogsSettings{}
	}

	var cfg HelmValuesMap
	if values, ok := configmap.Data["staging-values.json"]; ok {
		if err := json.Unmarshal([]byte(values), &cfg); err != nil {
			log.Errorw("failed to parse the staging configuration, staging logs are not persisted", "error", err)
			return application.StagingLogsSettings{}
		}
	}

	return application.NewStagingLogsSettings(cfg.PersistLogs, cfg.PersistLogsMaxSizeMB)
}

// schedulePersistStagingLogsWhenDone starts one server-owned wait that persists
// the staging logs when the jobs finish. It is independent of request handlers
// and client connections, so multiple waiters do not trigger multiple saves and
// a disconnect before complete does not skip persistence.
//
// The wait lives only in this server process. If the server restarts while
// a job is still running, that run's logs are not persisted, and the logs
// endpoint returns the not-available error once the job is cleaned up.
func schedulePersistStagingLogsWhenDone(
	cluster *kubernetes.Cluster,
	namespace, stageID string,
	jobs []batchv1.Job,
) {
	if len(jobs) == 0 {
		return
	}

	jobsCopy := append([]batchv1.Job(nil), jobs...)
	go func() {
		ctx := context.Background()
		log := requestctx.Logger(ctx)

		if err := waitForStagingJobsDone(ctx, cluster, jobsCopy); err != nil {
			log.Errorw(
				"staging log persistence wait failed",
				"error",
				err,
				"namespace",
				namespace,
				"stageID",
				stageID,
			)
			return
		}

		persistFinishedStagingLogs(ctx, cluster, jobsCopy, namespace, stageID)
	}()
}

// waitForStagingJobsDone waits until every job has finished, successfully
// or not. Unlike waitForStagingCompletion it is not bounded by the build
// timeout, because a slow build still deserves its logs. It keeps waiting
// across timeouts and stops on any other error, such as a deleted job.
func waitForStagingJobsDone(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	jobs []batchv1.Job,
) error {
	for _, job := range jobs {
		for {
			err := cluster.WaitForJobDone(
				ctx,
				helmchart.Namespace(),
				job.Name,
				duration.ToAppBuilt(),
			)

			if err == nil {
				break
			}

			if !wait.Interrupted(err) {
				return err
			}
		}
	}
	return nil
}

// persistFinishedStagingLogs keeps the logs of the finished staging run stageID
// when persisting is enabled. It is best effort: a failure is logged and does not
// change the outcome of the staging.
func persistFinishedStagingLogs(
	ctx context.Context,
	cluster *kubernetes.Cluster,
	jobs []batchv1.Job,
	namespace, stageID string,
) {
	if len(jobs) == 0 {
		return
	}

	settings := stagingLogsSettings(ctx, cluster)
	if !settings.Enabled {
		return
	}

	appName := jobs[0].Labels["app.kubernetes.io/name"]
	err := application.PersistStagingLogs(ctx, cluster, settings, namespace, appName, stageID)
	if err != nil {
		requestctx.Logger(ctx).Errorw("failed to persist the staging logs",
			"error", err, "namespace", namespace, "app", appName, "stageID", stageID)
	}
}

// serveStoredStagingLogs streams the persisted logs of the staging run stageID
// over a websocket, once the staging Job is gone. Without stored logs for the
// stage the not-available error is returned. Query filters (tail/since/container)
// are applied before sending. When follow is true the socket stays open after
// the replay and honors UI filter_params updates the same way streamLogs does.
func serveStoredStagingLogs(
	c *gin.Context,
	cluster *kubernetes.Cluster,
	namespace, stageID string,
	logParams *application.LogParameters,
) {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	body, found, err := application.OpenStagingLogs(ctx, cluster, namespace, stageID)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}
	if !found {
		response.Error(c, apierror.NewAPIError(stagingLogsUnavailableMessage, http.StatusNotFound))
		return
	}
	defer func() {
		_ = body.Close()
	}()

	allLines, err := readStoredStagingLogLines(body)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}

	filtered, err := application.FilterStoredStagingLogs(allLines, logParams)
	if err != nil {
		response.Error(c, apierror.NewBadRequestError(err.Error()))
		return
	}

	upgrader := newUpgrader()
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}
	defer func() {
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		_ = conn.Close()
	}()

	if err := writeStoredStagingLogLines(conn, filtered); err != nil {
		log.Errorw("failed to write the stored staging logs to the websocket", "error", err)
		return
	}

	follow := logParams != nil && logParams.Follow
	if !follow {
		return
	}

	// Keep the socket open after replay and apply mid-session filter updates
	// from the UI (filter_params), matching the live logs websocket.
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var update LogParameterUpdate
		if err := json.Unmarshal(message, &update); err != nil {
			log.Errorw("failed to unmarshal stored staging log parameter update", "error", err)
			continue
		}
		if update.Type != "filter_params" {
			continue
		}

		parsedParams, err := parseLogParameterUpdate(update)
		if err != nil {
			log.Errorw("failed to parse stored staging log parameter update", "error", err)
			continue
		}
		if err := validateContainerFilterPatterns(parsedParams); err != nil {
			log.Errorw("invalid stored staging log parameter update", "error", err)
			continue
		}

		filtered, err := application.FilterStoredStagingLogs(allLines, parsedParams)
		if err != nil {
			log.Errorw("failed to filter stored staging logs", "error", err)
			continue
		}

		// Do not tell the UI to clear its logs until the update has been fully
		// parsed, validated and applied. Otherwise a bad update leaves the UI
		// waiting for a FILTER_COMPLETE marker that can never arrive.
		if err := writeStoredStagingLogMarker(conn, "___FILTER_START___"); err != nil {
			log.Errorw("failed to send filter start marker for stored staging logs", "error", err)
			return
		}
		if err := writeStoredStagingLogLines(conn, filtered); err != nil {
			log.Errorw("failed to write filtered stored staging logs", "error", err)
			return
		}

		if !parsedParams.Follow {
			if err := writeStoredStagingLogMarker(conn, "___FILTER_COMPLETE___"); err != nil {
				log.Errorw("failed to send filter complete marker for stored staging logs", "error", err)
				return
			}
		}
	}
}

func writeStoredStagingLogMarker(conn *websocket.Conn, message string) error {
	return writeStoredStagingLogLines(conn, []tailer.ContainerLogLine{{Message: message}})
}

func writeStoredStagingLogLines(conn *websocket.Conn, lines []tailer.ContainerLogLine) error {
	for _, line := range lines {
		msg, err := json.Marshal(line)
		if err != nil {
			return err
		}
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return err
		}
	}
	return nil
}

func readStoredStagingLogLines(body io.Reader) ([]tailer.ContainerLogLine, error) {
	var lines []tailer.ContainerLogLine
	reader := bufio.NewReader(body)
	for {
		line, readErr := reader.ReadBytes('\n')
		if data := bytes.TrimSuffix(line, []byte("\n")); len(data) > 0 {
			var entry tailer.ContainerLogLine
			if err := json.Unmarshal(data, &entry); err != nil {
				return nil, err
			}
			lines = append(lines, entry)
		}
		if readErr == io.EOF {
			return lines, nil
		}
		if readErr != nil {
			return nil, readErr
		}
	}
}
