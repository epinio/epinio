package v1

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/epinio/epinio/helpers/kubernetes"
	"github.com/epinio/epinio/helpers/kubernetes/tailer"
	"github.com/epinio/epinio/helpers/tracelog"
	"github.com/epinio/epinio/internal/api/v1/models"
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/cli/clients/gitea"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type ApplicationsController struct {
	conn *websocket.Conn
}

func (hc ApplicationsController) Create(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return OrgIsNotKnown(org)
	}

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	var createRequest models.ApplicationCreateRequest
	err = json.Unmarshal(bodyBytes, &createRequest)
	if err != nil {
		return BadRequest(err)
	}

	appRef := models.NewAppRef(createRequest.Name, org)
	found, err := application.Exists(ctx, cluster, appRef)
	if err != nil {
		return InternalError(err, "failed to check for app resource")
	}
	if found {
		return AppAlreadyKnown(createRequest.Name)
	}

	err = application.Create(ctx, cluster, appRef)
	if err != nil {
		return InternalError(err)
	}
	return nil
}

func (hc ApplicationsController) Index(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return OrgIsNotKnown(org)
	}

	apps, err := application.ListApps(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	js, err := json.Marshal(apps)
	if err != nil {
		return InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func (hc ApplicationsController) Show(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	appName := params.ByName("app")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return OrgIsNotKnown(org)
	}

	exists, err = application.Exists(ctx, cluster, models.NewAppRef(appName, org))
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return AppIsNotKnown(appName)
	}

	// Application exists. It may not have a workload however.

	app, err := application.Lookup(ctx, cluster, org, appName)
	if err != nil {
		return InternalError(err)
	}
	if app == nil {
		// While the app exists, it has no workload.
		// Return something barebones.
		app = models.NewApp(appName, org)
		app.Status = `Inactive, without workload. Launch via "epinio app push"`
	}

	js, err := json.Marshal(app)
	if err != nil {
		return InternalError(err)
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

func (hc ApplicationsController) Update(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	appName := params.ByName("app")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return OrgIsNotKnown(org)
	}

	exists, err = application.Exists(ctx, cluster, models.NewAppRef(appName, org))
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return AppIsNotKnown(appName)
	}

	// Application exists. It may not have a workload however.

	app, err := application.Lookup(ctx, cluster, org, appName)
	if err != nil {
		return InternalError(err)
	}

	if app == nil {
		// App without workload cannot be scaled at the moment.
		// TODO: Extend to stash the request in the app or attached resource
		return NewAPIError("Unable to scale application without workload", "", http.StatusBadRequest)
	}

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	var updateRequest models.UpdateAppRequest
	err = json.Unmarshal(bodyBytes, &updateRequest)
	if err != nil {
		return BadRequest(err)
	}

	if updateRequest.Instances < 0 {
		return NewBadRequest("instances param should be integer equal or greater than zero")
	}

	workload := application.NewWorkload(cluster, app.AppRef())
	err = workload.Scale(r.Context(), updateRequest.Instances)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
func (hc ApplicationsController) Logs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	appName := params.ByName("app")
	stageID := params.ByName("stage_id")

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		jsonErrorResponse(w, InternalError(err))
		return
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		jsonErrorResponse(w, InternalError(err))
		return
	}

	if !exists {
		jsonErrorResponse(w, OrgIsNotKnown(org))
		return
	}

	if appName != "" {
		exists, err = application.Exists(ctx, cluster, models.NewAppRef(appName, org))
		if err != nil {
			jsonErrorResponse(w, InternalError(err))
			return
		}

		if !exists {
			jsonErrorResponse(w, AppIsNotKnown(appName))
			return
		}

		app, err := application.Lookup(ctx, cluster, org, appName)
		if err != nil {
			jsonErrorResponse(w, InternalError(err))
			return
		}
		if app == nil {
			// While app exists it has no workload
			jsonErrorResponse(w, NewAPIError("No logs available for application without workload", "", http.StatusBadRequest))
			return
		}
	}

	if appName == "" && stageID == "" {
		jsonErrorResponse(w, BadRequest(errors.New("You need to speficy either the stage id or the app")))
		return
	}

	queryValues := r.URL.Query()
	followStr := queryValues.Get("follow")

	var upgrader = websocket.Upgrader{}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		jsonErrorResponse(w, InternalError(err))
		return
	}

	follow := false
	if followStr == "true" {
		follow = true
	}

	log := tracelog.Logger(ctx)

	hc.conn = conn
	err = hc.streamPodLogs(ctx, org, appName, stageID, cluster, follow)
	if err != nil {
		log.V(1).Error(err, "error occured after upgrading the websockets connection")
		return
	}
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
func (hc ApplicationsController) streamPodLogs(ctx context.Context, orgName, appName, stageID string, cluster *kubernetes.Cluster, follow bool) error {
	logger := tracelog.NewLogger().WithName("streaming-logs-to-websockets").V(1)
	logChan := make(chan tailer.ContainerLogLine)
	logCtx, logCancelFunc := context.WithCancel(ctx)
	var wg sync.WaitGroup

	wg.Add(1)
	go func(outerWg *sync.WaitGroup) {
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

	// Send logs received on logChan to the websockets connection until either
	// logChan is closed or websocket connection is closed.
	for logLine := range logChan {
		msg, err := json.Marshal(logLine)
		if err != nil {
			return err
		}

		err = hc.conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			logger.Error(err, "failed to write to websockets")

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				hc.conn.Close()
				return nil
			}
			if websocket.IsUnexpectedCloseError(err) {
				hc.conn.Close()
				logger.Error(err, "websockets connection unexpectedly closed")
				return nil
			}

			normalCloseErr := hc.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{})
			if normalCloseErr != nil {
				err = errors.Wrap(err, normalCloseErr.Error())
			}

			abnormalCloseErr := hc.conn.Close()
			if abnormalCloseErr != nil {
				err = errors.Wrap(err, abnormalCloseErr.Error())
			}

			return err
		}
	}

	if err := hc.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{}); err != nil {
		return err
	}

	return hc.conn.Close()
}

func (hc ApplicationsController) Delete(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	appName := params.ByName("app")

	gitea, err := gitea.New(ctx)
	if err != nil {
		return InternalError(err)
	}

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	exists, err := organizations.Exists(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return OrgIsNotKnown(org)
	}

	appRef := models.NewAppRef(appName, org)
	found, err := application.Exists(ctx, cluster, appRef)
	if err != nil {
		return InternalError(err)
	}
	if !found {
		return AppIsNotKnown(appName)
	}

	app, err := application.Lookup(ctx, cluster, appRef.Org, appRef.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return InternalError(err)
	}

	response := models.ApplicationDeleteResponse{}
	if app != nil {
		response.UnboundServices = app.BoundServices
	}

	err = application.Delete(ctx, cluster, gitea, appRef)
	if err != nil {
		return InternalError(err)
	}

	js, err := json.Marshal(response)
	if err != nil {
		return InternalError(err)
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(js)
	if err != nil {
		return InternalError(err)
	}

	return nil
}
