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

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	// MaxTailLines is the maximum number of log lines that can be requested via the tail parameter
	// This prevents excessive memory usage and ensures reasonable response times
	MaxTailLines = 10000
)

// parseLogParameters parses and validates log query parameters
func parseLogParameters(tailStr, sinceStr, sinceTimeStr string) (*application.LogParameters, error) {
	params := &application.LogParameters{}

	if tailStr != "" {
		if tail, err := strconv.ParseInt(tailStr, 10, 64); err == nil {
			if tail < 0 {
				return nil, fmt.Errorf("tail parameter must be non-negative, got: %d", tail)
			}
			if tail > MaxTailLines {
				return nil, fmt.Errorf("tail parameter exceeds maximum of %d lines, got: %d", MaxTailLines, tail)
			}
			params.Tail = &tail
		} else {
			return nil, fmt.Errorf("invalid tail parameter: %s", tailStr)
		}
	}

	if sinceStr != "" {
		if since, err := time.ParseDuration(sinceStr); err == nil {
			if since < 0 {
				return nil, fmt.Errorf("since parameter must be non-negative, got: %s", since)
			}
			params.Since = &since
		} else {
			return nil, fmt.Errorf("invalid since parameter: %s", sinceStr)
		}
	}

	if sinceTimeStr != "" {
		if sinceTime, err := time.Parse(time.RFC3339, sinceTimeStr); err == nil {
			// Note: We allow future times here as they will be handled by returning no logs
			// This is better UX than rejecting the request
			params.SinceTime = &sinceTime
		} else {
			return nil, fmt.Errorf("invalid since_time parameter: %s (must be RFC3339 format)", sinceTimeStr)
		}
	}

	return params, nil
}

// ParseLogParametersForTest is a test helper that exposes parseLogParameters for testing
func ParseLogParametersForTest(tailStr, sinceStr, sinceTimeStr string) (*application.LogParameters, error) {
	return parseLogParameters(tailStr, sinceStr, sinceTimeStr)
}

// Logs handles the API endpoints GET /namespaces/:namespace/applications/:app/logs
// and	GET /namespaces/:namespace/staging/:stage_id/logs
// It arranges for the logs of the specified application to be
// streamed over a websocket. Dependent on the endpoint this may be
// either regular logs, or the app's staging logs.
func Logs(c *gin.Context) {
	ctx := c.Request.Context()
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")
	appName := c.Param("app")
	stageID := c.Param("stage_id")

	log.Infow("get cluster client")
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}

	if appName != "" {
		log.Infow("retrieve application", "name", appName, "namespace", namespace)

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
			response.Error(c, apierror.NewAPIError("No logs available for application without workload", http.StatusBadRequest))
			return
		}
	}

	if appName == "" && stageID == "" {
		response.Error(c, apierror.NewBadRequestError("you need to specify either the stage id or the app"))
		return
	}

	log.Infow("process query")

	followStr := c.Query("follow")
	tailStr := c.Query("tail")
	sinceStr := c.Query("since")
	sinceTimeStr := c.Query("since_time")

	// Parse and validate log parameters
	logParams, err := parseLogParameters(tailStr, sinceStr, sinceTimeStr)
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
	log.Infow("parsed log parameters",
		"tail", logParams.Tail,
		"since", logParams.Since,
		"since_time", logParams.SinceTime,
		"follow", logParams.Follow,
		"follow_raw", followStr)

	log.Infow("upgrade to web socket")

	var upgrader = newUpgrader()
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}

	log.Infow("streaming mode", "follow", logParams.Follow)
	log.Infow("streaming begin")

	err = streamPodLogs(ctx, conn, namespace, appName, stageID, cluster, logParams)
	if err != nil {
		log.Debugw("error occurred after upgrading the websockets connection", "error", err)
		return
	}

	log.Infow("streaming completed")
}

// streamPodLogs sends the logs of any containers matching namespaceName, appName
// and stageID to hc.conn (websockets) until ctx is Done or the connection is
// closed.
// Internally this uses two concurrent "threads" talking with each other
// over the logChan. This is a channel of ContainerLogLine.
// The first thread runs `application.Logs` in a go routine. It spins up a number of supporting go routines
// that are stopped when the passed context is "Done()". The parent go routine
// waits until all the subordinate routines are stopped. It does this by waiting on a WaitGroup.
// When that happens the parent go routine closes the logChan. This signals
// the main "thread" to also stop.
// The second (and main) thread reads the logChan and sends the received log messages over to the
// websocket connection. It returns either when the channel is closed or when the
// connection is closed. In any case it will call the cancel func that will stop
// all the children go routines described above and then will wait for their parent
// go routine to stop too (using another WaitGroup).
func streamPodLogs(ctx context.Context, conn *websocket.Conn, namespaceName, appName, stageID string, cluster *kubernetes.Cluster, logParams *application.LogParameters) error {
	logger := requestctx.Logger(ctx).With("component", "streamer-to-websockets")
	logChan := make(chan tailer.ContainerLogLine)
	logCtx, logCancelFunc := context.WithCancel(ctx)
	var wg sync.WaitGroup

	wg.Add(1)
	go func(outerWg *sync.WaitGroup) {
		logger.Debugw("create backend", "follow", logParams.Follow, "app", appName, "stage", stageID, "namespace", namespaceName)
		defer func() {
			logger.Debugw("backend ends")
		}()

		var tailWg sync.WaitGroup
		err := application.Logs(logCtx, logChan, &tailWg, cluster, appName, stageID, namespaceName, logParams)
		if err != nil {
			logger.Errorw("setting up log routines failed", "error", err)
		}

		logger.Debugw("wait for backend completion", "follow", logParams.Follow, "app", appName, "stage", stageID, "namespace", namespaceName)
		tailWg.Wait()  // Wait until all child routines are stopped
		close(logChan) // Close the channel so the loop below can stop
		outerWg.Done() // Let the outer method know we are done
	}(&wg)

	defer func() {
		logCancelFunc() // Just in case return some error, out of the normal flow.
		wg.Wait()
	}()

	logger.Debugw("stream copying begin")

	// Send logs received on logChan to the websockets connection until either
	// logChan is closed or websocket connection is closed.
	for logLine := range logChan {
		logger.Debugw("streaming", "log line", logLine)

		msg, err := json.Marshal(logLine)
		if err != nil {
			return err
		}

		err = conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			logger.Errorw("failed to write to websockets", "error", err)

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				connectionCloseError := conn.Close()

				if connectionCloseError != nil {
					return connectionCloseError
				}

				return nil
			}
			if websocket.IsUnexpectedCloseError(err) {
				connectionCloseError := conn.Close()

				if connectionCloseError != nil {
					return connectionCloseError
				}

				logger.Errorw("websockets connection unexpectedly closed", "error", err)
				return nil
			}

			normalCloseErr := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
			if normalCloseErr != nil {
				err = errors.Wrap(err, normalCloseErr.Error())
			}

			abnormalCloseErr := conn.Close()
			if abnormalCloseErr != nil {
				err = errors.Wrap(err, abnormalCloseErr.Error())
			}

			return err
		}
	}

	logger.Debugw("stream copying done")
	logger.Debugw("websocket teardown")

	if err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{}); err != nil {
		return err
	}

	return conn.Close()
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
