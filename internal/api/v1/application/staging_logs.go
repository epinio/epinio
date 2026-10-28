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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/internal/helmchart"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	batchv1 "k8s.io/api/batch/v1"
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
// over a websocket, once the staging Job is gone. Each stored line is already the
// JSON of one log line, so it is sent as it is. Without stored logs for the stage
// the not-available error is returned.
func serveStoredStagingLogs(c *gin.Context, cluster *kubernetes.Cluster, namespace, stageID string) {
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

	reader := bufio.NewReader(body)
	for {
		line, readErr := reader.ReadBytes('\n')
		if data := bytes.TrimSuffix(line, []byte("\n")); len(data) > 0 {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Errorw("failed to write the stored staging logs to the websocket", "error", err)
				return
			}
		}

		if readErr == io.EOF {
			return
		}
		if readErr != nil {
			log.Errorw("failed to read the stored staging logs", "error", readErr)
			return
		}
	}
}
