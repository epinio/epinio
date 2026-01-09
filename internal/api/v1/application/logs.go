// Copyright © 2021 - 2023 SUSE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//	http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"github.com/gorilla/websocket"
)

const (
	// MaxTailLines is the maximum number of log lines that can be requested via the tail parameter
	// This prevents excessive memory usage and ensures reasonable response times
	MaxTailLines = 100000
)

type LogParameterUpdate struct {
	Type   string `json:"type"`
	Params struct {
		Since     string `json:"since"`
		SinceTime string `json:"since_time"`
		Tail      int    `json:"tail"`
		Follow    bool   `json:"follow"`
	} `json:"params"`
}

// ParseLogParameters parses and validates log query parameters
func ParseLogParameters(
	tailStr,
	sinceStr,
	sinceTimeStr string,
) (*application.LogParameters, error) {
	params := &application.LogParameters{}

	if tailStr != "" {
		if tail, err := strconv.ParseInt(tailStr, 10, 64); err == nil {
			if tail < 0 {
				return nil, fmt.Errorf(
					"tail parameter must be non-negative, got: %d",
					tail,
				)
			}
			if tail > MaxTailLines {
				return nil, fmt.Errorf(
					"tail parameter exceeds maximum of %d lines, got: %d",
					MaxTailLines,
					tail,
				)
			}
			params.Tail = &tail
		} else {
			return nil, fmt.Errorf("invalid tail parameter: %s", tailStr)
		}
	}

	if sinceStr != "" {
		if since, err := time.ParseDuration(sinceStr); err == nil {
			if since < 0 {
				return nil, fmt.Errorf(
					"since parameter must be non-negative, got: %s",
					since,
				)
			}
			params.Since = &since
		} else {
			return nil, fmt.Errorf("invalid since parameter: %s", sinceStr)
		}
	}

	if sinceTimeStr != "" {
		if sinceTime, err := time.Parse(time.RFC3339, sinceTimeStr); err == nil {
			// Note: We allow future times here as they will be handled by returning
			// no logs. This is better UX than rejecting the request
			params.SinceTime = &sinceTime
		} else {
			return nil, fmt.Errorf(
				"invalid since_time parameter: %s (must be RFC3339 format)",
				sinceTimeStr,
			)
		}
	}

	return params, nil
}

// Logs handles the API endpoints GET /namespaces/:namespace/applications/:app/logs
// and	GET /namespaces/:namespace/staging/:stage_id/logs
// It arranges for the logs of the specified application to be
// streamed over a websocket. Dependent on the endpoint this may be
// either regular logs, or the app's staging logs.
//
// There is also support for dynamic updating of log parameters via
// the websocket connection. The client can send a JSON message with tail,
// since, and since_time fields to update the log filtering parameters.
func Logs(c *gin.Context) {
	ctx := c.Request.Context()

	namespace := c.Param("namespace")
	appName := c.Param("app")
	stageID := c.Param("stage_id")

	helpers.Logger.Debug("get cluster client")
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}

	if appName != "" {
		helpers.Logger.Debug("retrieve application", "name", appName, "namespace", namespace)

		app, err := application.Lookup(ctx, cluster, namespace, appName)
		if err != nil {
			response.Error(c, apierror.InternalError(err))
			return
		}

		if app == nil {
			response.Error(c, apierror.AppIsNotKnown(appName))
			return
		}

		if app.Workload == nil {
			// While the app exists it has no workload, therefore no logs
			response.Error(
				c,
				apierror.NewAPIError(
					"No logs available for application without workload",
					http.StatusBadRequest,
				),
			)
			return
		}
	}

	if appName == "" && stageID == "" {
		response.Error(
			c,
			apierror.NewBadRequestError("you need to specify either the stage id or the app"),
		)
		return
	}

	helpers.Logger.Debug("process query")

	// Extract query parameters
	followStr := c.Query("follow")
	tailStr := c.Query("tail")
	sinceStr := c.Query("since")
	sinceTimeStr := c.Query("since_time")

	// Parse and validate log parameters
	logParams, err := ParseLogParameters(tailStr, sinceStr, sinceTimeStr)
	if err != nil {
		response.Error(c, apierror.NewBadRequestError(err.Error()))
		return
	}

	// Set follow parameter
	follow := followStr == "true"
	if logParams == nil {
		logParams = &application.LogParameters{}
	}
	logParams.Follow = follow

	// Log the parsed parameters for debugging
	helpers.Logger.Debug(
		"parsed log parameters | ",
		"tail: ", logParams.Tail,
		"since: ", logParams.Since,
		"since_time: ", logParams.SinceTime,
		"follow: ", logParams.Follow,
		"follow_raw: ", followStr)

	helpers.Logger.Debug("upgrade to web socket")

	var upgrader = newUpgrader()
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}

	helpers.Logger.Debug("streaming mode", "follow", logParams.Follow)
	helpers.Logger.Debug("streaming begin")

	// Start streaming logs, if there is an error, return after logging it
	err = streamPodLogs(
		ctx,
		conn,
		namespace,
		appName,
		stageID,
		cluster,
		logParams,
	)
	if err != nil {
		helpers.Logger.Error(
			err,
			"error occurred after upgrading the websockets connection",
		)
		return
	}

	helpers.Logger.Debug("streaming completed")
}

/*
streamPodLogs sends the logs of any containers matching namespaceName, appName
and stageID to hc.conn (websockets) until ctx is Done or the connection is
closed.

Internally this uses two concurrent "threads" talking with each other
over the logChan. This is a channel of ContainerLogLine.

If the filtering parameters are updated a new log streaming goroutine is
started and the previous one is cancelled. The logChan is shared between
the goroutines to prevent the need for reconnecting the websocket.
*/
func streamPodLogs(
	ctx context.Context,
	conn *websocket.Conn,
	namespaceName,
	appName,
	stageID string,
	cluster *kubernetes.Cluster,
	logParams *application.LogParameters,
) error {
	logCtx, logCancelFunc := context.WithCancel(ctx)
	logChan := make(chan tailer.ContainerLogLine)
	var wg sync.WaitGroup
	var logWg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(
					err,
					websocket.CloseNormalClosure,
					websocket.CloseGoingAway,
				) {
					helpers.Logger.Debug("websocket closed normally")
				} else {
					helpers.Logger.Error(err, "error reading websocket message")
				}
				logCancelFunc() // Stop log streaming
				return
			}

			var update LogParameterUpdate
			if err := json.Unmarshal(message, &update); err != nil {
				helpers.Logger.Error(err, "failed to unmarshal parameter update")
				continue
			}

			if update.Type == "filter_params" {
				helpers.Logger.Debug(
					"received parameter update | ",
					"params: ",
					update.Params,
				)

				// Cancel current log streaming
				logCancelFunc()
				logWg.Wait()

				if len(logChan) > 0 {
					<-logChan
				}

				// Start new streaming with updated parameters
				logCtx, logCancelFunc = context.WithCancel(ctx)

				// Make sure we have valid parameters before continuing, if not, log
				// the error and ignore the update
				parsedParams, parsedParamsError := ParseLogParameters(
					strconv.Itoa(update.Params.Tail),
					update.Params.Since,
					update.Params.SinceTime,
				)

				if parsedParamsError != nil {
					helpers.Logger.Error(
						parsedParamsError,
						"failed to parse updated log parameters",
					)
					continue
				}

				// Set follow to false for updates, as these are one-off requests
				update.Params.Follow = false

				logWg.Add(1)
				go startLogStreaming(
					&logWg,
					logCtx,
					logChan,
					cluster,
					appName,
					stageID,
					namespaceName,
					parsedParams,
				)
			}
		}
	}()

	// Start initial log streaming
	logWg.Add(1)
	go startLogStreaming(
		&logWg,
		logCtx,
		logChan,
		cluster,
		appName,
		stageID,
		namespaceName,
		logParams,
	)

	defer func() {
		logCancelFunc()
		wg.Wait()
		logWg.Wait()
		close(logChan)
	}()

	helpers.Logger.Debug("stream copying begin")

	for logLine := range logChan {
		helpers.Logger.Debug("streaming", "log line", logLine)

		msg, err := json.Marshal(logLine)
		if err != nil {
			return err
		}

		err = conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			helpers.Logger.Error(err, "failed to write to websockets")

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return conn.Close()
			}
			if websocket.IsUnexpectedCloseError(err) {
				helpers.Logger.Error(err, "websockets connection unexpectedly closed")
				return conn.Close()
			}

			return err
		}
	}

	helpers.Logger.Debug("stream copying done")
	helpers.Logger.Debug("websocket teardown")

	if err := conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure,
			"",
		), time.Time{}); err != nil {
		return err
	}

	return conn.Close()
}

// startLogStreaming starts a goroutine to stream logs with the given parameters
// and writes them to logChan. It signals completion on wg when done.
func startLogStreaming(
	wg *sync.WaitGroup,
	ctx context.Context,
	logChan chan tailer.ContainerLogLine,
	cluster *kubernetes.Cluster,
	appName,
	stageID,
	namespaceName string,
	logParams *application.LogParameters,
) {

	defer func() {
		helpers.Logger.Info("backend ends")

		// Indicate end of log stream if not following
		if !logParams.Follow {
			logChan <- tailer.ContainerLogLine{
				Message:       "___FILTER_COMPLETE___",
				ContainerName: "",
				PodName:       "",
				Namespace:     "",
				Timestamp:     "",
			}
		}

		wg.Done()
	}()

	helpers.Logger.Debug(
		"create backend | ",
		"follow: ",
		logParams.Follow,
		"app: ",
		appName,
		"stage: ",
		stageID,
		"namespace: ",
		namespaceName,
	)

	var tailWg sync.WaitGroup
	err := application.Logs(
		ctx,
		logChan,
		&tailWg,
		cluster,
		appName,
		stageID,
		namespaceName,
		logParams,
	)
	if err != nil {
		helpers.Logger.Error(err, "setting up log routines failed")
	}

	helpers.Logger.Debug("wait for backend completion")
	tailWg.Wait()
}

// https://pkg.go.dev/github.com/gorilla/websocket#hdr-Origin_Considerations
// Regarding matching accessControlAllowOrigin and origin header:
// https: //developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Origin
func newUpgrader() websocket.Upgrader {
	allowedOrigins := viper.GetStringSlice("access-control-allow-origin")
	return websocket.Upgrader{
		CheckOrigin: CheckOriginFunc(allowedOrigins),
	}
}

func CheckOriginFunc(allowedOrigins []string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		originHeader := r.Header.Get("Origin")

		if originHeader == "" {
			return true
		}

		if len(allowedOrigins) == 0 {
			return true
		}

		for _, allowedOrigin := range allowedOrigins {
			trimmedOrigin := strings.TrimSuffix(allowedOrigin, "/")
			trimmedHeader := strings.TrimSuffix(originHeader, "/")
			if trimmedOrigin == "*" || trimmedOrigin == trimmedHeader {
				return true
			}
		}

		return false
	}
}
