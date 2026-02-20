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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/internal/cli/server/requestctx"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/gorilla/websocket"
)

var (
	// MaxTailLines is the maximum number of log lines that can be requested via the tail parameter
	// This prevents excessive memory usage and ensures reasonable response times
	MaxTailLines int64 = 100000
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
// includeContainersStr and excludeContainersStr are optional parameters for container filtering
func ParseLogParameters(
	tailStr,
	sinceStr,
	sinceTimeStr string,
	includeContainersStr ...string,
) (*application.LogParameters, error) {
	var excludeContainersStr string
	if len(includeContainersStr) > 1 {
		excludeContainersStr = includeContainersStr[1]
	}
	var actualIncludeContainersStr string
	if len(includeContainersStr) > 0 {
		actualIncludeContainersStr = includeContainersStr[0]
	}
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

	// Parse container filtering parameters
	if actualIncludeContainersStr != "" {
		split := strings.Split(actualIncludeContainersStr, ",")
		params.IncludeContainers = make([]string, 0, len(split))
		for _, container := range split {
			trimmed := strings.TrimSpace(container)
			if trimmed != "" {
				params.IncludeContainers = append(params.IncludeContainers, trimmed)
			}
		}
	}

	if excludeContainersStr != "" {
		split := strings.Split(excludeContainersStr, ",")
		params.ExcludeContainers = make([]string, 0, len(split))
		for _, container := range split {
			trimmed := strings.TrimSpace(container)
			if trimmed != "" {
				params.ExcludeContainers = append(params.ExcludeContainers, trimmed)
			}
		}
	}

	return params, nil
}

// validateContainerFilterPatterns validates that container filter patterns are valid regex
// This is called before upgrading to websocket so errors can be returned as HTTP errors
// Note: This mirrors the pattern building logic in application.Logs() to ensure consistency
func validateContainerFilterPatterns(logParams *application.LogParameters) error {
	if logParams == nil {
		return nil
	}

	// Validate include_containers patterns
	// Determine if user has specified include_containers (after filtering empty strings)
	// This affects whether default exclusion is applied in validation
	var hasUserIncludeFilter bool
	if len(logParams.IncludeContainers) > 0 {
		// Filter out empty strings and build pattern
		validIncludes := make([]string, 0, len(logParams.IncludeContainers))
		for _, container := range logParams.IncludeContainers {
			if trimmed := strings.TrimSpace(container); trimmed != "" {
				validIncludes = append(validIncludes, trimmed)
			}
		}

		if len(validIncludes) == 0 {
			// All containers were empty strings - this is an error
			// Match the behavior in application.Logs()
			return fmt.Errorf("include_containers parameter contains no valid container names")
		}

		// User has valid include filter
		hasUserIncludeFilter = true

		// Build pattern similar to application.Logs
		escapedIncludes := make([]string, len(validIncludes))
		for i, container := range validIncludes {
			if strings.ContainsAny(container, ".*+?^$|[]{}()\\") {
				escapedIncludes[i] = container
			} else {
				escapedIncludes[i] = regexp.QuoteMeta(container)
			}
		}
		pattern := strings.Join(escapedIncludes, "|")
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid include_containers pattern: %w", err)
		}
	}

	// Validate exclude_containers patterns
	// Only include default exclusions if user hasn't specified include_containers
	// This matches the behavior in application.Logs()
	var excludePatterns []string
	if !hasUserIncludeFilter {
		excludePatterns = []string{"linkerd-(proxy|init)"}
	}

	if len(logParams.ExcludeContainers) > 0 {
		if excludePatterns == nil {
			excludePatterns = []string{}
		}
		for _, container := range logParams.ExcludeContainers {
			trimmed := strings.TrimSpace(container)
			if trimmed == "" {
				continue
			}
			if strings.ContainsAny(trimmed, ".*+?^$|[]{}()\\") {
				excludePatterns = append(excludePatterns, trimmed)
			} else {
				excludePatterns = append(excludePatterns, regexp.QuoteMeta(trimmed))
			}
		}
	}

	if len(excludePatterns) > 0 {
		pattern := strings.Join(excludePatterns, "|")
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid exclude_containers pattern: %w", err)
		}
	}

	return nil
}

// ParseLogParametersForTest is a test helper that exposes ParseLogParameters for testing with container filters
func ParseLogParametersForTest(tailStr, sinceStr, sinceTimeStr string, includeContainersStr, excludeContainersStr string) (*application.LogParameters, error) {
	return ParseLogParameters(tailStr, sinceStr, sinceTimeStr, includeContainersStr, excludeContainersStr)
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
	log := requestctx.Logger(ctx)

	namespace := c.Param("namespace")
	appName := c.Param("app")
	stageID := c.Param("stage_id")

	log.Debugw("get cluster client")
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}

	if appName != "" {
		log.Debugw("retrieve application", "name", appName, "namespace", namespace)

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

	log.Debugw("process query")

	// Extract query parameters
	followStr := c.Query("follow")
	tailStr := c.Query("tail")
	sinceStr := c.Query("since")
	sinceTimeStr := c.Query("since_time")
	includeContainersStr := c.Query("include_containers")
	excludeContainersStr := c.Query("exclude_containers")

	// Parse and validate log parameters
	logParams, err := ParseLogParameters(tailStr, sinceStr, sinceTimeStr, includeContainersStr, excludeContainersStr)
	if err != nil {
		response.Error(c, apierror.NewBadRequestError(err.Error()))
		return
	}

	// Set follow parameter
	follow := followStr == "true"
	logParams.Follow = follow

	// Validate container filter regex patterns before upgrading to websocket
	// This allows us to return HTTP errors instead of silently failing
	if err := validateContainerFilterPatterns(logParams); err != nil {
		response.Error(c, apierror.NewBadRequestError(err.Error()))
		return
	}

	// Log the parsed parameters for debugging
	log.Debug(
		"parsed log parameters | ",
		"tail: ", logParams.Tail,
		"since: ", logParams.Since,
		"since_time: ", logParams.SinceTime,
		"follow: ", logParams.Follow,
		"follow_raw: ", followStr,
		"include_containers: ", logParams.IncludeContainers,
		"exclude_containers: ", logParams.ExcludeContainers)

	log.Debugw("upgrade to web socket")

	var upgrader = newUpgrader()
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}

	log.Debugw("streaming mode", "follow", logParams.Follow)
	log.Debugw("streaming begin")

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
		log.Errorw(
			"error occurred after upgrading the websockets connection",
			"error", err,
		)
		return
	}

	log.Debugw("streaming completed")
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
	log := requestctx.Logger(ctx)
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
					log.Debugw("websocket closed normally")
				} else {
					log.Errorw("error reading websocket message", "error", err)
				}
				logCancelFunc() // Stop log streaming
				return
			}

			var update LogParameterUpdate
			if err := json.Unmarshal(message, &update); err != nil {
				log.Errorw("failed to unmarshal parameter update", "error", err)
				continue
			}

			if update.Type == "filter_params" {
				log.Debugw("received parameter update",
					"params", update.Params,
				)

				// Send marker directly to WebSocket to tell frontend to clear logs
				// We do this BEFORE cancelling to ensure it arrives before any buffered messages
				startMarker := tailer.ContainerLogLine{
					Message:       "___FILTER_START___",
					ContainerName: "",
					PodName:       "",
					Namespace:     "",
					Timestamp:     "",
				}
				if msg, err := json.Marshal(startMarker); err == nil {
					if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
						log.Error(err, "failed to send filter start marker")
					}
				}

				// Cancel current log streaming
				logCancelFunc()
				logWg.Wait()

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
					log.Errorw("failed to parse updated log parameters",
						"error", parsedParamsError,
					)
					continue
				}

				// Use the follow parameter from the client
				parsedParams.Follow = update.Params.Follow

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

	log.Debugw("stream copying begin")

	for logLine := range logChan {
		log.Debugw("streaming", "log line", logLine)

		msg, err := json.Marshal(logLine)
		if err != nil {
			return err
		}

		err = conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Errorw("failed to write to websockets", "error", err)

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return conn.Close()
			}
			if websocket.IsUnexpectedCloseError(err) {
				connectionCloseError := conn.Close()

				if connectionCloseError != nil {
					return connectionCloseError
				}

				log.Errorw(
					"websockets connection unexpectedly closed",
					"error",
					err,
				)
				return nil
			}

			normalCloseErr := conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure,
					"",
				),
				time.Time{},
			)
			if normalCloseErr != nil {
				err = errors.Wrap(err, normalCloseErr.Error())
			}

			abnormalCloseErr := conn.Close()
			if abnormalCloseErr != nil {
				err = errors.Wrap(err, abnormalCloseErr.Error())
				log.Errorw("websockets connection unexpectedly closed", "error", err)
				return conn.Close()
			}

			return err
		}
	}

	log.Debugw("stream copying done")
	log.Debugw("websocket teardown")

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
	log := requestctx.Logger(ctx)

	defer func() {
		log.Infow("backend ends")

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

	log.Debugw("create backend",
		"follow", logParams.Follow,
		"app", appName,
		"stage", stageID,
		"namespace", namespaceName,
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
		log.Errorw("setting up log routines failed", "error", err)
	}

	log.Debugw("wait for backend completion")
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
