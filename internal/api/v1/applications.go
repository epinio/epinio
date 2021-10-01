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
	"github.com/epinio/epinio/internal/application"
	"github.com/epinio/epinio/internal/duration"
	epinioerrors "github.com/epinio/epinio/internal/errors"
	"github.com/epinio/epinio/internal/organizations"
	"github.com/epinio/epinio/internal/services"
	"github.com/epinio/epinio/pkg/api/core/v1/models"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

// ApplicationsController represents all functionality of the API related to applications
type ApplicationsController struct {
	conn *websocket.Conn
}

// Create handles the API endpoint POST /namespaces/:org/applications
// It creates a new and empty application. I.e. without a workload.
func (hc ApplicationsController) Create(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	username, err := GetUsername(r)
	if err != nil {
		return UserNotFound()
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

	// Sanity check the services, if any. IOW anything to be bound
	// has to exist now.  We will check again when the application
	// is deployed, to guard against bound services being removed
	// from now till then. While it should not be possible through
	// epinio itself (*), external editing of the relevant
	// resources cannot be excluded from consideration.
	//
	// (*) `epinio service delete S` on a bound service S will
	//      either reject the operation, or, when forced, unbind S
	//      from the app.

	var theIssues []APIError

	for _, serviceName := range createRequest.Configuration.Services {
		_, err := services.Lookup(ctx, cluster, org, serviceName)
		if err != nil {
			if err.Error() == "service not found" {
				theIssues = append(theIssues, ServiceIsNotKnown(serviceName))
				continue
			}

			theIssues = append([]APIError{InternalError(err)}, theIssues...)
			return MultiError{theIssues}
		}
	}

	if len(theIssues) > 0 {
		return MultiError{theIssues}
	}

	// Arguments found OK, now we can modify the system state

	err = application.Create(ctx, cluster, appRef, username)
	if err != nil {
		return InternalError(err)
	}

	desired := DefaultInstances
	if createRequest.Configuration.Instances != nil {
		desired = *createRequest.Configuration.Instances
	}

	err = application.ScalingSet(ctx, cluster, appRef, desired)
	if err != nil {
		return InternalError(err)
	}

	// Save service information.
	err = application.BoundServicesSet(ctx, cluster, appRef,
		createRequest.Configuration.Services, true)
	if err != nil {
		return InternalError(err)
	}

	// TODO: 643 Save EV variables

	err = jsonResponse(w, models.ResponseOK)
	if err != nil {
		return InternalError(err)
	}
	return nil
}

// Index handles the API endpoint GET /applications
// It lists all the known applications, with and without workload.
func (hc ApplicationsController) FullIndex(w http.ResponseWriter, r *http.Request) APIErrors {
	ctx := r.Context()

	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		return InternalError(err)
	}

	// naive: list namespaces, then all apps in each namespace ...
	// can we query kube for all apps directly, across namespaces ...
	// needs label selector, pods, and app CRD resources (workloads, and undeployed apps).
	// the naive way seems to be much easier to implement right now.

	orgList, err := organizations.List(ctx, cluster)
	if err != nil {
		return InternalError(err)
	}

	var allApps models.AppList

	for _, org := range orgList {
		apps, err := application.List(ctx, cluster, org.Name)
		if err != nil {
			if _, ok := err.(epinioerrors.NamespaceMissingError); ok {
				continue
			}
			return InternalError(err)
		}

		allApps = append(allApps, apps...)
	}

	js, err := json.Marshal(allApps)
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

// Index handles the API endpoint GET /namespaces/:org/applications
// It lists all the known applications, with and without workload.
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

	apps, err := application.List(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	err = jsonResponse(w, apps)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// Show handles the API endpoint GET /namespaces/:org/applications/:app
// It returns the details of the specified application.
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

	app, err := application.Lookup(ctx, cluster, org, appName)
	if err != nil {
		return InternalError(err)
	}

	if app == nil {
		return AppIsNotKnown(appName)
	}

	err = jsonResponse(w, app)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// ServiceApps handles the API endpoint GET /namespaces/:org/serviceapps
// It returns a map from services to the apps they are bound to, in
// the specified org.  Internally it asks each app in the org for its
// bound services and then inverts that map to get the desired result.
func (hc ApplicationsController) ServiceApps(w http.ResponseWriter, r *http.Request) APIErrors {
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

	appsOf, err := servicesToApps(ctx, cluster, org)
	if err != nil {
		return InternalError(err)
	}

	err = jsonResponse(w, appsOf)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// Update handles the API endpoint PATCH /namespaces/:org/applications/:app
// It modifies the specified application. Currently this is only the
// number of instances to run.
func (hc ApplicationsController) Update(w http.ResponseWriter, r *http.Request) APIErrors { // nolint:gocyclo // simplification defered
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	appName := params.ByName("app")
	username, err := GetUsername(r)
	if err != nil {
		return UserNotFound()
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

	exists, err = application.Exists(ctx, cluster, models.NewAppRef(appName, org))
	if err != nil {
		return InternalError(err)
	}

	if !exists {
		return AppIsNotKnown(appName)
	}

	// Retrieve and validate update request ...

	defer r.Body.Close()
	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return InternalError(err)
	}

	var updateRequest models.ApplicationUpdateRequest
	err = json.Unmarshal(bodyBytes, &updateRequest)
	if err != nil {
		return BadRequest(err)
	}

	if updateRequest.Instances != nil && *updateRequest.Instances < 0 {
		return NewBadRequest("instances param should be integer equal or greater than zero")
	}

	app, err := application.Lookup(ctx, cluster, org, appName)
	if err != nil {
		return InternalError(err)
	}

	// TODO: Can we optimize to perform a single restart regardless of what changed ?!
	// TODO: Should we ?

	if updateRequest.Instances != nil {
		desired := *updateRequest.Instances

		// Save to configuration
		err := application.ScalingSet(ctx, cluster, app.Meta, desired)
		if err != nil {
			return InternalError(err)
		}

		// Restart workload, if any
		if app.Workload != nil {
			err = application.NewWorkload(cluster, app.Meta).Scale(ctx, desired)
			if err != nil {
				return InternalError(err)
			}
		}
	}

	if len(updateRequest.Environment) > 0 {
		err := application.EnvironmentSet(ctx, cluster, app.Meta, updateRequest.Environment, true)
		if err != nil {
			return InternalError(err)
		}

		// Restart workload, if any
		if app.Workload != nil {
			// For this read the new set of variables back
			varNames, err := application.EnvironmentNames(ctx, cluster, app.Meta)
			if err != nil {
				return InternalError(err)
			}

			err = application.NewWorkload(cluster, app.Meta).
				EnvironmentChange(ctx, varNames)
			if err != nil {
				return InternalError(err)
			}
		}
	}

	if len(updateRequest.Services) > 0 {
		// Take old state
		oldBound, err := application.BoundServiceNameSet(ctx, cluster, app.Meta)
		if err != nil {
			return InternalError(err)
		}

		var theIssues []APIError
		var okToBind []string

		for _, serviceName := range updateRequest.Services {
			_, err := services.Lookup(ctx, cluster, org, serviceName)
			if err != nil {
				if err.Error() == "service not found" {
					theIssues = append(theIssues, ServiceIsNotKnown(serviceName))
					continue
				}

				theIssues = append([]APIError{InternalError(err)}, theIssues...)
				return MultiError{theIssues}
			}

			okToBind = append(okToBind, serviceName)
		}

		err = application.BoundServicesSet(ctx, cluster, app.Meta, okToBind, true)
		if err != nil {
			theIssues = append([]APIError{InternalError(err)}, theIssues...)
			return MultiError{theIssues}
		}

		// Restart workload, if any
		if app.Workload != nil {
			// For this read the new set of bound services back,
			// as full service structures
			newBound, err := application.BoundServices(ctx, cluster, app.Meta)
			if err != nil {
				theIssues = append([]APIError{InternalError(err)}, theIssues...)
				return MultiError{theIssues}
			}

			err = application.NewWorkload(cluster, app.Meta).
				BoundServicesChange(ctx, username, oldBound, newBound)
			if err != nil {
				theIssues = append([]APIError{InternalError(err)}, theIssues...)
				return MultiError{theIssues}
			}
		}

		if len(theIssues) > 0 {
			return MultiError{theIssues}
		}
	}

	err = jsonResponse(w, models.ResponseOK)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// Running handles the API endpoint GET /namespaces/:org/applications/:app/running
// It waits for the specified application to be running (i.e. its
// deployment to be complete), before it returns. An exception is if
// the application does not become running without
// `duration.ToAppBuilt()` (default: 10 minutes). In that case it
// returns with an error after that time.
func (hc ApplicationsController) Running(w http.ResponseWriter, r *http.Request) APIErrors {
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

	app, err := application.Lookup(ctx, cluster, org, appName)
	if err != nil {
		return InternalError(err)
	}

	if app == nil {
		return AppIsNotKnown(appName)
	}

	if app.Workload == nil {
		// While the app exists it has no workload, and therefore no status
		return NewAPIError("No status available for application without workload",
			"", http.StatusBadRequest)
	}

	err = cluster.WaitForDeploymentCompleted(
		ctx, nil, org, appName, duration.ToAppBuilt())
	if err != nil {
		return InternalError(err)
	}

	err = jsonResponse(w, models.ResponseOK)
	if err != nil {
		return InternalError(err)
	}
	return nil
}

// Logs handles the API endpoints GET /namespaces/:org/applications/:app/logs
// and                            GET /namespaces/:org/staging/:stage_id/logs
// It arranges for the logs of the specified application to be
// streamed over a websocket. Dependent on the endpoint this may be
// either regular logs, or the app's staging logs.
func (hc ApplicationsController) Logs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	params := httprouter.ParamsFromContext(ctx)
	org := params.ByName("org")
	appName := params.ByName("app")
	stageID := params.ByName("stage_id")
	log := tracelog.Logger(ctx)

	log.Info("get cluster client")
	cluster, err := kubernetes.GetCluster(ctx)
	if err != nil {
		jsonErrorResponse(w, InternalError(err))
		return
	}

	log.Info("validate organization", "name", org)
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
		log.Info("retrieve application", "name", appName, "org", org)

		app, err := application.Lookup(ctx, cluster, org, appName)
		if err != nil {
			jsonErrorResponse(w, InternalError(err))
			return
		}

		if app == nil {
			jsonErrorResponse(w, AppIsNotKnown(appName))
			return
		}

		if app.Workload == nil {
			// While the app exists it has no workload, therefore no logs
			jsonErrorResponse(w, NewAPIError("No logs available for application without workload", "", http.StatusBadRequest))
			return
		}
	}

	if appName == "" && stageID == "" {
		jsonErrorResponse(w, BadRequest(errors.New("You need to specify either the stage id or the app")))
		return
	}

	log.Info("process query")
	queryValues := r.URL.Query()
	followStr := queryValues.Get("follow")

	log.Info("processed query", "values", queryValues)
	log.Info("upgrade to web socket")

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

	log.Info("streaming mode", "follow", follow)
	log.Info("streaming begin")

	hc.conn = conn
	err = hc.streamPodLogs(ctx, org, appName, stageID, cluster, follow)
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
func (hc ApplicationsController) streamPodLogs(ctx context.Context, orgName, appName, stageID string, cluster *kubernetes.Cluster, follow bool) error {
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

	logger.Info("stream copying done")
	logger.Info("websocket teardown")

	if err := hc.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Time{}); err != nil {
		return err
	}

	return hc.conn.Close()
}

// Delete handles the API endpoint DELETE /namespaces/:org/applications/:app
// It removes the named application
func (hc ApplicationsController) Delete(w http.ResponseWriter, r *http.Request) APIErrors {
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

	app := models.NewAppRef(appName, org)

	found, err := application.Exists(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}
	if !found {
		return AppIsNotKnown(appName)
	}

	services, err := application.BoundServiceNames(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	response := models.ApplicationDeleteResponse{UnboundServices: services}

	err = application.Delete(ctx, cluster, app)
	if err != nil {
		return InternalError(err)
	}

	err = jsonResponse(w, response)
	if err != nil {
		return InternalError(err)
	}

	return nil
}

// GetUsername returns the username from the header
func GetUsername(r *http.Request) (string, error) {
	username := r.Header.Get("X-Webauth-User")
	if len(username) <= 0 {
		return "", errors.New("username not found in the header")
	}

	return username, nil
}
