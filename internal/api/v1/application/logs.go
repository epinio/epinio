package application

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/response"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/organizations"
	apierror "github.com/epinio/epinio/pkg/api/core/v1/errors"
	"github.com/gin-gonic/gin"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// Logs handles the API endpoints GET /namespaces/:org/applications/:app/logs
// and                            GET /namespaces/:org/staging/:stage_id/logs
// It arranges for the logs of the specified application to be
// streamed over a websocket. Dependent on the endpoint this may be
// either regular logs, or the app's staging logs.
func (hc Controller) Logs(c *gin.Context) {
	ctx := c.Request.Context()
	org := c.Param("org")
	appName := c.Param("app")
	stageID := c.Param("stage_id")
	log := tracelog.Logger(ctx)

	log.Info("get cluster client")
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}

	log.Info("validate organization", "name", org)
	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}

	if !exists {
		response.Error(c, apierror.OrgIsNotKnown(org))
		return
	}

	if appName != "" {
		log.Info("retrieve application", "name", appName, "org", org)

		app, err := application.Lookup(ctx, cluster, org, appName)
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
			response.Error(c, apierror.NewAPIError("No logs available for application without workload", "", http.StatusBadRequest))
			return
		}
	}

	if appName == "" && stageID == "" {
		response.Error(c, apierror.BadRequest(errors.New("You need to specify either the stage id or the app")))
		return
	}

	log.Info("process query")

	followStr := c.Query("follow")

	log.Info("upgrade to web socket")

	var upgrader = websocket.Upgrader{}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		response.Error(c, apierror.InternalError(err))
		return
	}

	follow := followStr == "true"

	log.Info("streaming mode", "follow", follow)
	log.Info("streaming begin")

	err = hc.streamPodLogs(ctx, conn, org, appName, stageID, cluster, follow)
	if err != nil {
		log.V(1).Error(err, "error occurred after upgrading the websockets connection")
		return
	}

	log.Info("streaming completed")
}

// streamPodLogs sends the logs of any containers matching orgName, appName
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
func (hc Controller) streamPodLogs(ctx context.Context, conn *websocket.Conn, orgName, appName, stageID string, cluster *kubernetes.Cluster, follow bool) error {
	logger := tracelog.NewLogger().WithName("streamer-to-websockets").V(1)
	logChan := make(chan tailer.ContainerLogLine)
	logCtx, logCancelFunc := context.WithCancel(ctx)
	var wg sync.WaitGroup

	wg.Add(1)
	go func(outerWg *sync.WaitGroup) {
		logger.Info("create backend")
		defer func() {
			logger.Info("backend ends")
		}()

		var tailWg sync.WaitGroup
		err := application.Logs(logCtx, logChan, &tailWg, cluster, follow, appName, stageID, orgName)
		if err != nil {
			logger.Error(err, "setting up log routines failed")
		}
		tailWg.Wait()  // Wait until all child routines are stopped
		close(logChan) // Close the channel so the loop below can stop
		outerWg.Done() // Let the outer method know we are done
	}(&wg)

	defer func() {
		logCancelFunc() // Just in case return some error, out of the normal flow.
		wg.Wait()
	}()

	logger.Info("stream copying begin")

	// Send logs received on logChan to the websockets connection until either
	// logChan is closed or websocket connection is closed.
	for logLine := range logChan {
		msg, err := json.Marshal(logLine)
		if err != nil {
			return err
		}

		err = conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			logger.Error(err, "failed to write to websockets")

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				conn.Close()
				return nil
			}
			if websocket.IsUnexpectedCloseError(err) {
				conn.Close()
				logger.Error(err, "websockets connection unexpectedly closed")
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

	logger.Info("stream copying done")
	logger.Info("websocket teardown")

	if err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{}); err != nil {
		return err
	}

	return conn.Close()
}
